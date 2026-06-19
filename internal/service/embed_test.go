package service

import (
	"context"
	"errors"
	"testing"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"

	"gorm.io/gorm"
)

func TestEmbedServiceCreatesRevealableEncryptedSecret(t *testing.T) {
	codec, err := secret.NewAESGCMCodec("unit-test-secret")
	if err != nil {
		t.Fatalf("init secret codec: %v", err)
	}
	repo := &embedFakeRepository{}
	service := NewEmbedService(repo, nil, nil, "token-secret", codec)

	result, err := service.CreateApp(context.Background(), CreateEmbedAppInput{
		TenantID:  1,
		ProjectID: 2,
		Name:      "经营看板助手",
	})
	if err != nil {
		t.Fatalf("create embed app: %v", err)
	}
	if result.AppSecret == "" || result.App.SecretHash == "" {
		t.Fatalf("expected generated app secret and hash, got %+v", result)
	}
	if result.App.SecretCiphertext == "" || result.App.SecretCiphertext == result.AppSecret {
		t.Fatalf("expected encrypted app secret, got ciphertext=%q secret=%q", result.App.SecretCiphertext, result.AppSecret)
	}

	revealed, err := service.RevealAppSecret(context.Background(), 1, 2, result.App.ID)
	if err != nil {
		t.Fatalf("reveal app secret: %v", err)
	}
	if revealed.AppID != result.App.AppID || revealed.AppSecret != result.AppSecret {
		t.Fatalf("unexpected revealed secret: %+v", revealed)
	}
}

func TestEmbedServiceStatusManagement(t *testing.T) {
	repo := &embedFakeRepository{}
	service := NewEmbedService(repo, nil, nil, "token-secret")
	result, err := service.CreateApp(context.Background(), CreateEmbedAppInput{
		TenantID:  1,
		ProjectID: 2,
	})
	if err != nil {
		t.Fatalf("create embed app: %v", err)
	}

	if _, err := service.UpdateAppStatus(context.Background(), 1, 2, result.App.ID, "paused"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for unsupported status, got %v", err)
	}
	updated, err := service.UpdateAppStatus(context.Background(), 1, 2, result.App.ID, "disabled")
	if err != nil {
		t.Fatalf("disable app: %v", err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("expected disabled status, got %+v", updated)
	}

	_, err = service.CreateToken(context.Background(), CreateEmbedTokenInput{
		AppID:          result.App.AppID,
		AppSecret:      result.AppSecret,
		ExternalUserID: "third-party-user-1",
	})
	if !errors.Is(err, ErrEmbedAppDisabled) {
		t.Fatalf("expected disabled app to reject token creation, got %v", err)
	}
}

type embedFakeRepository struct {
	app     *model.EmbedApp
	deleted bool
}

func (r *embedFakeRepository) CreateApp(ctx context.Context, app *model.EmbedApp) error {
	if app.ID == 0 {
		app.ID = 1001
	}
	r.app = cloneEmbedApp(app)
	return nil
}

func (r *embedFakeRepository) ListApps(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]model.EmbedApp, int64, error) {
	if r.app == nil || r.deleted {
		return nil, 0, nil
	}
	return []model.EmbedApp{*cloneEmbedApp(r.app)}, 1, nil
}

func (r *embedFakeRepository) GetApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) (*model.EmbedApp, error) {
	if r.app == nil || r.deleted || r.app.TenantID != tenantID || r.app.ProjectID != projectID || r.app.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneEmbedApp(r.app), nil
}

func (r *embedFakeRepository) GetAppByAppID(ctx context.Context, appID string) (*model.EmbedApp, error) {
	if r.app == nil || r.deleted || r.app.AppID != appID {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneEmbedApp(r.app), nil
}

func (r *embedFakeRepository) UpdateAppStatus(ctx context.Context, tenantID uint64, projectID uint64, id uint64, status string) (*model.EmbedApp, error) {
	if r.app == nil || r.deleted || r.app.TenantID != tenantID || r.app.ProjectID != projectID || r.app.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	r.app.Status = status
	return cloneEmbedApp(r.app), nil
}

func (r *embedFakeRepository) DeleteApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) error {
	if r.app == nil || r.deleted || r.app.TenantID != tenantID || r.app.ProjectID != projectID || r.app.ID != id {
		return gorm.ErrRecordNotFound
	}
	r.deleted = true
	return nil
}

func (r *embedFakeRepository) EnsureSession(ctx context.Context, input repository.EnsureEmbedSessionInput) (*model.EmbedSession, *model.ChatSession, error) {
	return nil, nil, nil
}

func (r *embedFakeRepository) GetSessionByChatSessionID(ctx context.Context, chatSessionID uint64) (*model.EmbedSession, error) {
	return nil, gorm.ErrRecordNotFound
}

func cloneEmbedApp(app *model.EmbedApp) *model.EmbedApp {
	if app == nil {
		return nil
	}
	copy := *app
	return &copy
}
