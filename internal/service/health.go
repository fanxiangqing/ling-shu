package service

import (
	"context"
	"time"

	"ling-shu/internal/database"

	"gorm.io/gorm"
)

type HealthService struct {
	db       *gorm.DB
	redis    RedisPinger
	exec     ExecHealthChecker
	execHard bool
}

type RedisPinger interface {
	Ping(ctx context.Context) error
}

type ExecHealthChecker interface {
	CheckHealth(ctx context.Context) error
}

type HealthOption func(*HealthService)

type HealthStatus struct {
	Status    string                 `json:"status"`
	CheckedAt time.Time              `json:"checked_at"`
	Checks    map[string]CheckStatus `json:"checks"`
}

type CheckStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func NewHealthService(db *gorm.DB, options ...HealthOption) *HealthService {
	service := &HealthService{db: db}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithRedisPinger(redis RedisPinger) HealthOption {
	return func(s *HealthService) {
		s.redis = redis
	}
}

func WithExecHealthChecker(exec ExecHealthChecker, hard bool) HealthOption {
	return func(s *HealthService) {
		s.exec = exec
		s.execHard = hard
	}
}

func (s *HealthService) Liveness() HealthStatus {
	return HealthStatus{
		Status:    "ok",
		CheckedAt: time.Now(),
		Checks: map[string]CheckStatus{
			"server": {Status: "ok"},
		},
	}
}

func (s *HealthService) Readiness(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:    "ok",
		CheckedAt: time.Now(),
		Checks:    map[string]CheckStatus{},
	}

	sqlDB, err := database.SQLDB(s.db)
	if err != nil {
		status.Status = "error"
		status.Checks["mysql"] = CheckStatus{Status: "error", Message: err.Error()}
		return status
	}
	if sqlDB == nil {
		status.Checks["mysql"] = CheckStatus{Status: "disabled"}
		return status
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		status.Status = "error"
		status.Checks["mysql"] = CheckStatus{Status: "error", Message: err.Error()}
		return status
	}

	status.Checks["mysql"] = CheckStatus{Status: "ok"}
	if s.redis != nil {
		redisCtx, redisCancel := context.WithTimeout(ctx, 2*time.Second)
		defer redisCancel()
		if err := s.redis.Ping(redisCtx); err != nil {
			status.Status = "error"
			status.Checks["redis"] = CheckStatus{Status: "error", Message: err.Error()}
			return status
		}
		status.Checks["redis"] = CheckStatus{Status: "ok"}
	}
	if s.exec != nil {
		execCtx, execCancel := context.WithTimeout(ctx, 2*time.Second)
		defer execCancel()
		if err := s.exec.CheckHealth(execCtx); err != nil {
			checkStatus := "degraded"
			if s.execHard {
				status.Status = "error"
				checkStatus = "error"
			}
			status.Checks["exec"] = CheckStatus{Status: checkStatus, Message: err.Error()}
			return status
		}
		status.Checks["exec"] = CheckStatus{Status: "ok"}
	}
	return status
}
