package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ling-shu/internal/llm"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
	"ling-shu/internal/repository"
)

const defaultRetrieveLimit = 50

type Retriever struct {
	knowledgeRepo repository.KnowledgeRepository
	embedder      llm.Provider
	vectorStore   VectorStore
	topK          int
}

type RetrieverOption func(*Retriever)

func WithEmbedder(embedder llm.Provider) RetrieverOption {
	return func(r *Retriever) {
		r.embedder = embedder
	}
}

func WithVectorStore(store VectorStore) RetrieverOption {
	return func(r *Retriever) {
		r.vectorStore = store
	}
}

func WithTopK(topK int) RetrieverOption {
	return func(r *Retriever) {
		r.topK = topK
	}
}

func NewRetriever(knowledgeRepo repository.KnowledgeRepository, opts ...RetrieverOption) *Retriever {
	retriever := &Retriever{
		knowledgeRepo: knowledgeRepo,
		topK:          defaultTopK,
	}
	for _, opt := range opts {
		opt(retriever)
	}
	if retriever.topK <= 0 {
		retriever.topK = defaultTopK
	}
	return retriever
}

func (r *Retriever) Retrieve(ctx context.Context, req Request) (*Context, error) {
	if req.TenantID == 0 || req.ProjectID == 0 {
		return nil, ErrInvalidRequest
	}
	out := &Context{}

	if r.knowledgeRepo != nil {
		lexicalContext, err := r.retrieveLexical(ctx, req)
		if err != nil {
			return nil, err
		}
		out.BusinessTerms = lexicalContext.BusinessTerms
		out.Metrics = lexicalContext.Metrics
		out.FewShots = lexicalContext.FewShots
	}

	vectorContext, err := r.retrieveVector(ctx, req)
	if err != nil {
		return nil, err
	}
	if vectorContext != nil {
		out.Hits = vectorContext.Hits
		out.BusinessTerms = mergeKnowledge(vectorContext.BusinessTerms, out.BusinessTerms)
		out.Metrics = mergeKnowledge(vectorContext.Metrics, out.Metrics)
		out.FewShots = mergeFewShots(vectorContext.FewShots, out.FewShots)
	}
	return out, nil
}

func (r *Retriever) retrieveLexical(ctx context.Context, req Request) (*Context, error) {
	enabled := true
	page := repository.Page{Page: 1, PageSize: retrieveLimit(req.Limit)}
	filter := repository.KnowledgeFilter{
		TenantID:     req.TenantID,
		ProjectID:    req.ProjectID,
		DatasourceID: req.DatasourceID,
		Enabled:      &enabled,
	}

	terms, _, err := r.knowledgeRepo.ListTerms(ctx, filter, page)
	if err != nil {
		return nil, err
	}
	metrics, _, err := r.knowledgeRepo.ListMetrics(ctx, filter, page)
	if err != nil {
		return nil, err
	}
	fewShots, _, err := r.knowledgeRepo.ListFewShots(ctx, filter, page)
	if err != nil {
		return nil, err
	}

	return &Context{
		BusinessTerms: buildAgentTerms(rankTerms(terms, req.Question)),
		Metrics:       buildAgentMetrics(rankMetrics(metrics, req.Question)),
		FewShots:      buildAgentFewShots(rankFewShots(fewShots, req.Question)),
	}, nil
}

func (r *Retriever) retrieveVector(ctx context.Context, req Request) (*Context, error) {
	if r.embedder == nil || r.vectorStore == nil || strings.TrimSpace(req.Question) == "" {
		return nil, nil
	}
	if !r.embedder.Configured() {
		return nil, nil
	}
	embeddingResp, err := r.embedder.Embeddings(ctx, llm.EmbeddingRequest{
		Input: []string{req.Question},
	})
	if err != nil {
		return nil, err
	}
	if len(embeddingResp.Embeddings) == 0 || len(embeddingResp.Embeddings[0]) == 0 {
		return nil, nil
	}
	hits, err := r.vectorStore.Search(ctx, VectorSearchRequest{
		TenantID:     req.TenantID,
		ProjectID:    req.ProjectID,
		DatasourceID: req.DatasourceID,
		Vector:       float64ToFloat32(embeddingResp.Embeddings[0]),
		TopK:         vectorTopK(req.Limit, r.topK),
	})
	if err != nil {
		return nil, err
	}
	return contextFromHits(hits), nil
}

func retrieveLimit(limit int) int {
	if limit <= 0 {
		return defaultRetrieveLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func vectorTopK(limit int, fallback int) int {
	if limit > 0 && limit < fallback {
		return limit
	}
	if fallback > 0 {
		return fallback
	}
	return defaultTopK
}

func rankTerms(items []model.KBTerm, question string) []model.KBTerm {
	out := append([]model.KBTerm(nil), items...)
	sort.SliceStable(out, func(i int, j int) bool {
		return scoreText(termSearchText(out[i]), question) > scoreText(termSearchText(out[j]), question)
	})
	return out
}

func rankMetrics(items []model.KBMetric, question string) []model.KBMetric {
	out := append([]model.KBMetric(nil), items...)
	sort.SliceStable(out, func(i int, j int) bool {
		return scoreText(metricSearchText(out[i]), question) > scoreText(metricSearchText(out[j]), question)
	})
	return out
}

func rankFewShots(items []model.KBFewShotSQL, question string) []model.KBFewShotSQL {
	out := append([]model.KBFewShotSQL(nil), items...)
	sort.SliceStable(out, func(i int, j int) bool {
		return scoreText(fewShotSearchText(out[i]), question) > scoreText(fewShotSearchText(out[j]), question)
	})
	return out
}

func scoreText(text string, question string) int {
	text = strings.ToLower(strings.TrimSpace(text))
	question = strings.ToLower(strings.TrimSpace(question))
	if text == "" || question == "" {
		return 0
	}
	score := 0
	if strings.Contains(text, question) || strings.Contains(question, text) {
		score += 10
	}
	for _, token := range strings.Fields(question) {
		if strings.Contains(text, token) {
			score++
		}
	}
	for _, token := range strings.Fields(text) {
		if len([]rune(token)) >= 2 && strings.Contains(question, token) {
			score += 2
		}
	}
	return score
}

func termSearchText(term model.KBTerm) string {
	parts := []string{term.Term, term.Definition}
	parts = append(parts, aliasesFromJSON(term.AliasesJSON)...)
	return strings.Join(parts, " ")
}

func metricSearchText(metric model.KBMetric) string {
	return strings.Join([]string{metric.Name, metric.Description, metric.Formula, metric.DefaultTimeColumn}, " ")
}

func fewShotSearchText(fewShot model.KBFewShotSQL) string {
	return strings.Join([]string{fewShot.Question, fewShot.SQLText, fewShot.Explanation}, " ")
}

func aliasesFromJSON(content *string) []string {
	if content == nil || strings.TrimSpace(*content) == "" {
		return nil
	}
	var aliases []string
	if err := json.Unmarshal([]byte(*content), &aliases); err != nil {
		return nil
	}
	return aliases
}

func buildAgentTerms(terms []model.KBTerm) []query.AgentKnowledge {
	out := make([]query.AgentKnowledge, 0, len(terms))
	for _, term := range terms {
		out = append(out, query.AgentKnowledge{
			Name:        term.Term,
			Description: term.Definition,
		})
	}
	return out
}

func contextFromHits(hits []Hit) *Context {
	out := &Context{Hits: hits}
	for _, hit := range hits {
		switch hit.KBType {
		case KBTypeTerm:
			out.BusinessTerms = append(out.BusinessTerms, query.AgentKnowledge{
				Name:        hitLabel(hit, "业务术语", "术语", "term"),
				Description: hit.ChunkText,
			})
		case KBTypeMetric:
			out.Metrics = append(out.Metrics, query.AgentKnowledge{
				Name:        hitLabel(hit, "指标", "metric"),
				Description: hit.ChunkText,
				Expression:  extractLineValue(hit.ChunkText, "计算口径", "公式", "expression"),
			})
		case KBTypeFewShot:
			out.FewShots = append(out.FewShots, query.AgentFewShot{
				Question:     firstNonEmpty(extractLineValue(hit.ChunkText, "问题", "question"), hit.ChunkText),
				SQL:          extractLineValue(hit.ChunkText, "SQL", "sql"),
				DatasourceID: hit.DatasourceID,
			})
		}
	}
	return out
}

func hitLabel(hit Hit, prefixes ...string) string {
	if value := extractLineValue(hit.ChunkText, prefixes...); value != "" {
		return value
	}
	if hit.RefID > 0 {
		return fmt.Sprintf("%s#%d", hit.KBType, hit.RefID)
	}
	return hit.KBType
}

func extractLineValue(text string, prefixes ...string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range prefixes {
			prefix = strings.ToLower(strings.TrimSpace(prefix))
			for _, sep := range []string{":", "："} {
				marker := prefix + sep
				if strings.HasPrefix(lower, marker) {
					return strings.TrimSpace(trimmed[len(marker):])
				}
			}
		}
	}
	return ""
}

func mergeKnowledge(priority []query.AgentKnowledge, rest []query.AgentKnowledge) []query.AgentKnowledge {
	seen := make(map[string]struct{}, len(priority)+len(rest))
	out := make([]query.AgentKnowledge, 0, len(priority)+len(rest))
	for _, item := range append(priority, rest...) {
		key := strings.ToLower(strings.TrimSpace(item.Name + "\x00" + item.Expression + "\x00" + item.Description))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func mergeFewShots(priority []query.AgentFewShot, rest []query.AgentFewShot) []query.AgentFewShot {
	seen := make(map[string]struct{}, len(priority)+len(rest))
	out := make([]query.AgentFewShot, 0, len(priority)+len(rest))
	for _, item := range append(priority, rest...) {
		key := strings.ToLower(strings.TrimSpace(item.Question + "\x00" + item.SQL))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func float64ToFloat32(values []float64) []float32 {
	out := make([]float32, 0, len(values))
	for _, value := range values {
		out = append(out, float32(value))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func buildAgentMetrics(metrics []model.KBMetric) []query.AgentKnowledge {
	out := make([]query.AgentKnowledge, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, query.AgentKnowledge{
			Name:        metric.Name,
			Description: metric.Description,
			Expression:  metric.Formula,
		})
	}
	return out
}

func buildAgentFewShots(fewShots []model.KBFewShotSQL) []query.AgentFewShot {
	out := make([]query.AgentFewShot, 0, len(fewShots))
	for _, fewShot := range fewShots {
		out = append(out, query.AgentFewShot{
			Question:     fewShot.Question,
			SQL:          fewShot.SQLText,
			DatasourceID: fewShot.DatasourceID,
		})
	}
	return out
}
