package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type UserMemoryWorker struct {
	store     UserMemoryStore
	extractor UserMemoryExtractor
	semantic  UserMemorySemanticIndex
	interval  time.Duration
	logger    *zap.Logger
}

func NewUserMemoryWorker(store UserMemoryStore, extractor UserMemoryExtractor, semantic UserMemorySemanticIndex, logger *zap.Logger) *UserMemoryWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &UserMemoryWorker{store: store, extractor: extractor, semantic: semantic, interval: 10 * time.Second, logger: logger}
}

func (w *UserMemoryWorker) Start(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
					w.logger.Warn("user memory worker cycle failed", zap.Error(err))
				}
			}
		}
	}()
}

func (w *UserMemoryWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.store == nil {
		return nil
	}
	jobs, err := w.store.ClaimJobs(ctx, time.Now(), 10)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := w.process(ctx, job); err != nil {
			_ = w.store.FailJob(ctx, job.ID, err, time.Now().Add(time.Duration(job.Attempts)*time.Minute))
			continue
		}
		if err := w.store.CompleteJob(ctx, job.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *UserMemoryWorker) process(ctx context.Context, job UserMemoryJob) error {
	switch job.JobType {
	case "memory_index":
		if w.semantic == nil {
			return nil
		}
		item, err := w.store.Get(ctx, job.Scope, uint64MapValue(job.Payload, "memory_id"))
		if errors.Is(err, ErrUserMemoryNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if !item.IsUsable(time.Now()) {
			return w.semantic.Delete(ctx, item.Scope, item.ID)
		}
		metadata, err := w.semantic.Index(ctx, item)
		if err != nil {
			return err
		}
		return w.store.SetVectorMetadata(ctx, item.Scope, item.ID, metadata)
	case "memory_delete_index":
		if w.semantic == nil {
			return nil
		}
		return w.semantic.Delete(ctx, job.Scope, uint64MapValue(job.Payload, "memory_id"))
	case "memory_consolidate":
		if w.extractor == nil {
			return nil
		}
	default:
		return nil
	}
	input := UserTurnInput{
		Scope: job.Scope, SessionID: job.SessionID, AssistantMessageID: job.MessageID,
		Question: stringMapValue(job.Payload, "question"),
		Timezone: stringMapValue(job.Payload, "timezone"), OccurredAt: job.CreatedAt,
	}
	operations, err := w.extractor.Extract(ctx, input)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		now := time.Now()
		item := BuildUserMemory(job.Scope, operation, UserMemorySourceInferred, job.SessionID, job.MessageID, now)
		if item.Status != UserMemoryStatusActive {
			return fmt.Errorf("inferred memory must be active")
		}
		evidence := UserMemoryEvidence{
			Scope: item.Scope, SessionID: job.SessionID, MessageID: job.MessageID,
			EvidenceType: UserMemorySourceInferred, EvidenceSummary: compactUserMemoryText(item.Content, 180), CreatedAt: now,
		}
		event := UserMemoryEvent{
			Scope: item.Scope, Operation: "inferred", MemoryKey: item.Key,
			Metadata: map[string]any{"kind": item.Kind, "source": "worker", "auto_activated": true}, ActorUserID: job.Scope.UserID, CreatedAt: now,
		}
		saved, err := w.store.Upsert(ctx, item, evidence, event)
		if err != nil {
			return err
		}
		if w.semantic != nil && saved.ID > 0 {
			if err := w.store.EnqueueJob(ctx, UserMemoryJob{
				Scope: saved.Scope, JobType: "memory_index", Payload: map[string]any{"memory_id": saved.ID},
				Status: "pending", RunAfter: now,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func uint64MapValue(values map[string]any, key string) uint64 {
	switch value := values[key].(type) {
	case uint64:
		return value
	case int:
		if value > 0 {
			return uint64(value)
		}
	case int64:
		if value > 0 {
			return uint64(value)
		}
	case float64:
		if value > 0 {
			return uint64(value)
		}
	}
	return 0
}

func stringMapValue(values map[string]any, key string) string {
	if value, ok := values[key]; ok {
		return fmt.Sprint(value)
	}
	return ""
}
