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

func TestEmbedServiceServerBootstrapBypassesBrowserOrigin(t *testing.T) {
	repo := &embedFakeRepository{}
	service := NewEmbedService(repo, nil, nil, "token-secret")
	result, err := service.CreateApp(context.Background(), CreateEmbedAppInput{
		TenantID:       1,
		ProjectID:      2,
		AllowedOrigins: []string{"https://console.example.com"},
	})
	if err != nil {
		t.Fatalf("create embed app: %v", err)
	}
	token, err := service.CreateToken(context.Background(), CreateEmbedTokenInput{
		AppID:          result.App.AppID,
		AppSecret:      result.AppSecret,
		ExternalUserID: "third-party-user-1",
	})
	if err != nil {
		t.Fatalf("create browser token: %v", err)
	}

	_, err = service.Bootstrap(context.Background(), BootstrapEmbedInput{
		AppID:       result.App.AppID,
		AccessToken: token.AccessToken,
		SessionKey:  "dashboard:123",
	})
	if !errors.Is(err, ErrEmbedOriginDenied) {
		t.Fatalf("expected browser bootstrap to keep origin check, got %v", err)
	}

	session, access, err := service.BootstrapServer(context.Background(), BootstrapEmbedServerInput{
		AppID:            result.App.AppID,
		AppSecret:        result.AppSecret,
		ExternalUserID:   "third-party-user-1",
		ExternalUserName: "三方用户",
		SessionKey:       "dashboard:123",
	})
	if err != nil {
		t.Fatalf("server bootstrap: %v", err)
	}
	if session.SessionID == 0 || access == nil || access.EmbedSession == nil {
		t.Fatalf("expected server session and access, got session=%+v access=%+v", session, access)
	}
	if session.SessionKey != "dashboard:123" || session.Identity.ExternalUserID != "third-party-user-1" {
		t.Fatalf("unexpected server bootstrap result: %+v", session)
	}

	reused, _, err := service.BootstrapServer(context.Background(), BootstrapEmbedServerInput{
		AppID:          result.App.AppID,
		AppSecret:      result.AppSecret,
		ExternalUserID: "third-party-user-1",
		SessionKey:     "dashboard:123",
	})
	if err != nil {
		t.Fatalf("server bootstrap reused session: %v", err)
	}
	if reused.SessionID != session.SessionID {
		t.Fatalf("expected context session reuse, first=%d second=%d", session.SessionID, reused.SessionID)
	}
}

func TestEmbedServiceValidateServerSessionAccess(t *testing.T) {
	repo := &embedFakeRepository{}
	service := NewEmbedService(repo, nil, nil, "token-secret")
	app, err := service.CreateApp(context.Background(), CreateEmbedAppInput{
		TenantID:  1,
		ProjectID: 2,
	})
	if err != nil {
		t.Fatalf("create embed app: %v", err)
	}
	session, _, err := service.BootstrapServer(context.Background(), BootstrapEmbedServerInput{
		AppID:          app.App.AppID,
		AppSecret:      app.AppSecret,
		ExternalUserID: "u-1",
		SessionKey:     "customer:88",
	})
	if err != nil {
		t.Fatalf("server bootstrap: %v", err)
	}

	access, err := service.ValidateServerSessionAccess(context.Background(), ValidateEmbedServerSessionInput{
		AppID:          app.App.AppID,
		AppSecret:      app.AppSecret,
		ExternalUserID: "u-1",
		ChatSessionID:  session.SessionID,
	})
	if err != nil {
		t.Fatalf("validate server session: %v", err)
	}
	if access.EmbedSession.ChatSessionID != session.SessionID {
		t.Fatalf("unexpected access: %+v", access)
	}

	_, err = service.ValidateServerSessionAccess(context.Background(), ValidateEmbedServerSessionInput{
		AppID:          app.App.AppID,
		AppSecret:      app.AppSecret,
		ExternalUserID: "u-2",
		ChatSessionID:  session.SessionID,
	})
	if !errors.Is(err, ErrEmbedTokenInvalid) {
		t.Fatalf("expected mismatched external user to be rejected, got %v", err)
	}
}

type embedFakeRepository struct {
	app                *model.EmbedApp
	deleted            bool
	sessions           map[string]*model.EmbedSession
	chatSessions       map[uint64]*model.ChatSession
	nextEmbedSessionID uint64
	nextChatSessionID  uint64
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
	if input.App == nil || r.deleted {
		return nil, nil, gorm.ErrRecordNotFound
	}
	if r.sessions == nil {
		r.sessions = map[string]*model.EmbedSession{}
	}
	if r.chatSessions == nil {
		r.chatSessions = map[uint64]*model.ChatSession{}
	}
	key := input.App.AppID + "\x00" + input.ExternalUserID + "\x00" + input.SessionKey
	if input.SessionMode != "new" {
		if existing := r.sessions[key]; existing != nil && existing.Status == "active" {
			chat := r.chatSessions[existing.ChatSessionID]
			if chat != nil {
				return cloneEmbedSession(existing), cloneChatSession(chat), nil
			}
		}
	}
	if r.nextEmbedSessionID == 0 {
		r.nextEmbedSessionID = 2001
	}
	if r.nextChatSessionID == 0 {
		r.nextChatSessionID = 3001
	}
	userID := uint64(4001)
	chat := &model.ChatSession{
		TenantID:  input.App.TenantID,
		ProjectID: input.App.ProjectID,
		UserID:    userID,
		Title:     "内嵌会话",
		Status:    "active",
	}
	chat.ID = r.nextChatSessionID
	r.nextChatSessionID++
	session := &model.EmbedSession{
		TenantID:         input.App.TenantID,
		ProjectID:        input.App.ProjectID,
		EmbedAppID:       input.App.ID,
		AppID:            input.App.AppID,
		ExternalUserID:   input.ExternalUserID,
		ExternalUserName: input.ExternalUserName,
		SessionKey:       input.SessionKey,
		ChatSessionID:    chat.ID,
		UserID:           userID,
		Status:           "active",
	}
	session.ID = r.nextEmbedSessionID
	r.nextEmbedSessionID++
	r.sessions[key] = cloneEmbedSession(session)
	r.chatSessions[chat.ID] = cloneChatSession(chat)
	return session, chat, nil
}

func (r *embedFakeRepository) GetSessionByChatSessionID(ctx context.Context, chatSessionID uint64) (*model.EmbedSession, error) {
	for _, session := range r.sessions {
		if session.ChatSessionID == chatSessionID && session.Status == "active" {
			return cloneEmbedSession(session), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func cloneEmbedApp(app *model.EmbedApp) *model.EmbedApp {
	if app == nil {
		return nil
	}
	copy := *app
	return &copy
}

func cloneEmbedSession(session *model.EmbedSession) *model.EmbedSession {
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}

func cloneChatSession(session *model.ChatSession) *model.ChatSession {
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}
