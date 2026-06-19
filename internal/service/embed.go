package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"

	"go.uber.org/zap"
)

const (
	defaultEmbedTokenTTL = time.Hour
	maxEmbedTokenTTL     = 24 * time.Hour
	defaultEmbedKey      = "default"
)

type EmbedService struct {
	embedRepo      repository.EmbedRepository
	datasourceRepo repository.DatasourceRepository
	provider       *ProviderService
	tokenSecret    []byte
	secretCodec    secret.Codec
	logger         *zap.Logger
}

type CreateEmbedAppInput struct {
	TenantID       uint64
	ProjectID      uint64
	Name           string
	AllowedOrigins []string
	SessionPolicy  string
	LauncherTitle  string
	WelcomeMessage string
	CreatedBy      uint64
}

type CreateEmbedAppResult struct {
	App       *model.EmbedApp `json:"app"`
	AppSecret string          `json:"app_secret"`
}

type RevealEmbedAppSecretResult struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

type CreateEmbedTokenInput struct {
	AppID            string
	AppSecret        string
	ExternalUserID   string
	ExternalUserName string
	TTLSeconds       int
}

type EmbedTokenResult struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type BootstrapEmbedInput struct {
	AppID        string
	AccessToken  string
	SessionKey   string
	SessionMode  string
	ParentOrigin string
}

type EmbedBootstrapResult struct {
	App          EmbedAppView        `json:"app"`
	TenantID     uint64              `json:"tenant_id"`
	ProjectID    uint64              `json:"project_id"`
	UserID       uint64              `json:"user_id"`
	SessionID    uint64              `json:"session_id"`
	SessionKey   string              `json:"session_key"`
	Datasources  []EmbedDatasource   `json:"datasources"`
	Capabilities EmbedCapabilities   `json:"capabilities"`
	Identity     EmbedIdentityResult `json:"identity"`
}

type EmbedAppView struct {
	AppID          string   `json:"app_id"`
	Name           string   `json:"name"`
	LauncherTitle  string   `json:"launcher_title"`
	WelcomeMessage string   `json:"welcome_message,omitempty"`
	SessionPolicy  string   `json:"session_policy"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

type EmbedDatasource struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	DBType   string `json:"db_type"`
	Status   string `json:"status,omitempty"`
	SyncedAt string `json:"synced_at,omitempty"`
}

type EmbedCapabilities struct {
	ASR           bool `json:"asr"`
	TTS           bool `json:"tts"`
	RealtimeVoice bool `json:"realtime_voice"`
}

type EmbedIdentityResult struct {
	ExternalUserID   string `json:"external_user_id"`
	ExternalUserName string `json:"external_user_name,omitempty"`
}

type EmbedAccess struct {
	App            *model.EmbedApp
	EmbedSession   *model.EmbedSession
	ExternalUserID string
}

type embedTokenClaims struct {
	AppID            string `json:"app_id"`
	ExternalUserID   string `json:"external_user_id"`
	ExternalUserName string `json:"external_user_name,omitempty"`
	IssuedAt         int64  `json:"iat"`
	Expires          int64  `json:"exp"`
}

func NewEmbedService(embedRepo repository.EmbedRepository, datasourceRepo repository.DatasourceRepository, provider *ProviderService, tokenSecret string, codecs ...secret.Codec) *EmbedService {
	var codec secret.Codec = secret.PlainCodec{}
	if len(codecs) > 0 && codecs[0] != nil {
		codec = codecs[0]
	}
	return &EmbedService{
		embedRepo:      embedRepo,
		datasourceRepo: datasourceRepo,
		provider:       provider,
		tokenSecret:    []byte(tokenSecret),
		secretCodec:    codec,
		logger:         zap.NewNop(),
	}
}

func (s *EmbedService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *EmbedService) CreateApp(ctx context.Context, input CreateEmbedAppInput) (*CreateEmbedAppResult, error) {
	if s == nil || s.embedRepo == nil || input.TenantID == 0 || input.ProjectID == 0 {
		return nil, ErrInvalidInput
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "内嵌助手"
	}
	appID := "emb_" + randomToken(15)
	appSecret := "lsk_" + randomToken(32)
	secretCiphertext, err := encryptSecret(s.secretCodec, appSecret)
	if err != nil {
		return nil, ErrSecretEncryptFailed
	}
	origins, err := encodeStringList(normalizeOrigins(input.AllowedOrigins))
	if err != nil {
		return nil, err
	}
	app := &model.EmbedApp{
		TenantID:           input.TenantID,
		ProjectID:          input.ProjectID,
		AppID:              appID,
		Name:               name,
		SecretHash:         hashSecret(appSecret),
		SecretCiphertext:   secretCiphertext,
		AllowedOriginsJSON: optionalJSON(origins),
		SessionPolicy:      normalizeSessionPolicy(input.SessionPolicy),
		LauncherTitle:      firstNonEmptyService(strings.TrimSpace(input.LauncherTitle), "智能问数"),
		WelcomeMessage:     strings.TrimSpace(input.WelcomeMessage),
		Status:             "active",
		CreatedBy:          input.CreatedBy,
	}
	if err := s.embedRepo.CreateApp(ctx, app); err != nil {
		s.logger.Error("embed app create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}
	return &CreateEmbedAppResult{App: app, AppSecret: appSecret}, nil
}

func (s *EmbedService) RevealAppSecret(ctx context.Context, tenantID uint64, projectID uint64, id uint64) (*RevealEmbedAppSecretResult, error) {
	if s == nil || s.embedRepo == nil || tenantID == 0 || projectID == 0 || id == 0 {
		return nil, ErrInvalidInput
	}
	app, err := s.embedRepo.GetApp(ctx, tenantID, projectID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(app.SecretCiphertext) == "" {
		return nil, ErrSecretDecryptFailed
	}
	appSecret, err := decryptSecret(s.secretCodec, app.SecretCiphertext)
	if err != nil || strings.TrimSpace(appSecret) == "" {
		return nil, ErrSecretDecryptFailed
	}
	return &RevealEmbedAppSecretResult{
		AppID:     app.AppID,
		AppSecret: appSecret,
	}, nil
}

func (s *EmbedService) ListApps(ctx context.Context, tenantID uint64, projectID uint64, page int, pageSize int) (PageResult[model.EmbedApp], error) {
	if s == nil || s.embedRepo == nil || tenantID == 0 || projectID == 0 {
		return PageResult[model.EmbedApp]{}, ErrInvalidInput
	}
	p := NewPage(page, pageSize)
	items, total, err := s.embedRepo.ListApps(ctx, tenantID, projectID, p)
	if err != nil {
		return PageResult[model.EmbedApp]{}, err
	}
	return PageResult[model.EmbedApp]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *EmbedService) UpdateAppStatus(ctx context.Context, tenantID uint64, projectID uint64, id uint64, status string) (*model.EmbedApp, error) {
	if s == nil || s.embedRepo == nil || tenantID == 0 || projectID == 0 || id == 0 {
		return nil, ErrInvalidInput
	}
	status = strings.TrimSpace(status)
	if status != "active" && status != "disabled" {
		return nil, ErrInvalidInput
	}
	return s.embedRepo.UpdateAppStatus(ctx, tenantID, projectID, id, status)
}

func (s *EmbedService) DeleteApp(ctx context.Context, tenantID uint64, projectID uint64, id uint64) error {
	if s == nil || s.embedRepo == nil || tenantID == 0 || projectID == 0 || id == 0 {
		return ErrInvalidInput
	}
	return s.embedRepo.DeleteApp(ctx, tenantID, projectID, id)
}

func (s *EmbedService) CreateToken(ctx context.Context, input CreateEmbedTokenInput) (*EmbedTokenResult, error) {
	if s == nil || s.embedRepo == nil || len(s.tokenSecret) == 0 {
		return nil, ErrInvalidInput
	}
	app, err := s.embedRepo.GetAppByAppID(ctx, strings.TrimSpace(input.AppID))
	if err != nil {
		return nil, err
	}
	if app.Status != "active" {
		return nil, ErrEmbedAppDisabled
	}
	if !hmac.Equal([]byte(app.SecretHash), []byte(hashSecret(input.AppSecret))) {
		return nil, ErrEmbedSecretInvalid
	}
	externalUserID := trimRunes(strings.TrimSpace(input.ExternalUserID), 191)
	if externalUserID == "" {
		return nil, ErrInvalidInput
	}
	ttl := time.Duration(input.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultEmbedTokenTTL
	}
	if ttl > maxEmbedTokenTTL {
		ttl = maxEmbedTokenTTL
	}
	now := time.Now()
	claims := embedTokenClaims{
		AppID:            app.AppID,
		ExternalUserID:   externalUserID,
		ExternalUserName: trimRunes(strings.TrimSpace(input.ExternalUserName), 128),
		IssuedAt:         now.Unix(),
		Expires:          now.Add(ttl).Unix(),
	}
	token, err := s.signToken(claims)
	if err != nil {
		return nil, err
	}
	return &EmbedTokenResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   now.Add(ttl),
	}, nil
}

func (s *EmbedService) Bootstrap(ctx context.Context, input BootstrapEmbedInput) (*EmbedBootstrapResult, error) {
	claims, app, err := s.parseAccessToken(ctx, input.AccessToken)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.AppID) != "" && strings.TrimSpace(input.AppID) != app.AppID {
		return nil, ErrInvalidInput
	}
	if err := s.validateParentOrigin(app, input.ParentOrigin); err != nil {
		return nil, err
	}
	key, mode := resolveEmbedSession(app.SessionPolicy, input.SessionKey, input.SessionMode)
	embedSession, chatSession, err := s.embedRepo.EnsureSession(ctx, repository.EnsureEmbedSessionInput{
		App:              app,
		ExternalUserID:   claims.ExternalUserID,
		ExternalUserName: claims.ExternalUserName,
		SessionKey:       key,
		SessionMode:      mode,
	})
	if err != nil {
		return nil, err
	}
	datasources, err := s.projectDatasources(ctx, app.TenantID, app.ProjectID)
	if err != nil {
		return nil, err
	}
	return &EmbedBootstrapResult{
		App:          appView(app),
		TenantID:     app.TenantID,
		ProjectID:    app.ProjectID,
		UserID:       chatSession.UserID,
		SessionID:    chatSession.ID,
		SessionKey:   embedSession.SessionKey,
		Datasources:  datasources,
		Capabilities: s.capabilities(ctx, app.TenantID, app.ProjectID),
		Identity: EmbedIdentityResult{
			ExternalUserID:   claims.ExternalUserID,
			ExternalUserName: claims.ExternalUserName,
		},
	}, nil
}

func (s *EmbedService) ValidateSessionAccess(ctx context.Context, accessToken string, chatSessionID uint64) (*EmbedAccess, error) {
	claims, app, err := s.parseAccessToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if chatSessionID == 0 {
		return nil, ErrInvalidInput
	}
	embedSession, err := s.embedRepo.GetSessionByChatSessionID(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}
	if embedSession.EmbedAppID != app.ID || embedSession.ExternalUserID != claims.ExternalUserID {
		return nil, ErrEmbedTokenInvalid
	}
	return &EmbedAccess{
		App:            app,
		EmbedSession:   embedSession,
		ExternalUserID: claims.ExternalUserID,
	}, nil
}

func (s *EmbedService) projectDatasources(ctx context.Context, tenantID uint64, projectID uint64) ([]EmbedDatasource, error) {
	if s.datasourceRepo == nil {
		return nil, nil
	}
	items, _, err := s.datasourceRepo.ListByProject(ctx, tenantID, projectID, repository.Page{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	out := make([]EmbedDatasource, 0, len(items))
	for _, item := range items {
		out = append(out, EmbedDatasource{
			ID:       item.ID,
			Name:     item.Name,
			DBType:   item.DBType,
			Status:   item.Status,
			SyncedAt: datasourceSyncedAt(item.LastSyncAt, item.LastSyncStatus),
		})
	}
	return out, nil
}

func (s *EmbedService) capabilities(ctx context.Context, tenantID uint64, projectID uint64) EmbedCapabilities {
	if s.provider == nil {
		return EmbedCapabilities{}
	}
	summary := s.provider.SummaryWithScope(ctx, ProviderScopeInput{TenantID: tenantID, ProjectID: projectID})
	return EmbedCapabilities{
		ASR:           summary.ASR.Configured,
		TTS:           summary.TTS.Configured,
		RealtimeVoice: summary.ASR.Configured,
	}
}

func (s *EmbedService) parseAccessToken(ctx context.Context, token string) (*embedTokenClaims, *model.EmbedApp, error) {
	claims, err := s.parseToken(token)
	if err != nil {
		return nil, nil, err
	}
	app, err := s.embedRepo.GetAppByAppID(ctx, claims.AppID)
	if err != nil {
		return nil, nil, err
	}
	if app.Status != "active" {
		return nil, nil, ErrEmbedAppDisabled
	}
	return claims, app, nil
}

func (s *EmbedService) signToken(claims embedTokenClaims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerPart, err := encodeEmbedJSON(header)
	if err != nil {
		return "", err
	}
	claimsPart, err := encodeEmbedJSON(claims)
	if err != nil {
		return "", err
	}
	signingInput := headerPart + "." + claimsPart
	return signingInput + "." + s.sign(signingInput), nil
}

func (s *EmbedService) parseToken(token string) (*embedTokenClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 || len(s.tokenSecret) == 0 {
		return nil, ErrEmbedTokenInvalid
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(signingInput)), []byte(parts[2])) {
		return nil, ErrEmbedTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrEmbedTokenInvalid
	}
	var claims embedTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrEmbedTokenInvalid
	}
	if strings.TrimSpace(claims.AppID) == "" || strings.TrimSpace(claims.ExternalUserID) == "" {
		return nil, ErrEmbedTokenInvalid
	}
	if claims.Expires <= time.Now().Unix() {
		return nil, ErrEmbedTokenInvalid
	}
	return &claims, nil
}

func (s *EmbedService) sign(input string) string {
	mac := hmac.New(sha256.New, s.tokenSecret)
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *EmbedService) validateParentOrigin(app *model.EmbedApp, origin string) error {
	allowed := appView(app).AllowedOrigins
	if len(allowed) == 0 {
		return nil
	}
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return ErrEmbedOriginDenied
	}
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimRight(item, "/"), origin) {
			return nil
		}
	}
	return ErrEmbedOriginDenied
}

func encodeEmbedJSON(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(content), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func randomToken(bytes int) string {
	if bytes <= 0 {
		bytes = 16
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		now := sha256.Sum256([]byte(time.Now().String()))
		return base64.RawURLEncoding.EncodeToString(now[:])[:bytes]
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func normalizeOrigins(origins []string) []string {
	out := make([]string, 0, len(origins))
	seen := map[string]bool{}
	for _, origin := range origins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		out = append(out, origin)
	}
	return out
}

func encodeStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	content, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func optionalJSON(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeSessionPolicy(value string) string {
	switch strings.TrimSpace(value) {
	case "user", "context", "new":
		return strings.TrimSpace(value)
	default:
		return "context"
	}
}

func normalizeSessionMode(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "new" || value == "reuse" {
		return value
	}
	if strings.TrimSpace(fallback) == "new" {
		return "new"
	}
	return "reuse"
}

func resolveEmbedSession(policy string, key string, mode string) (string, string) {
	policy = normalizeSessionPolicy(policy)
	switch policy {
	case "user":
		if strings.TrimSpace(mode) == "new" {
			return defaultEmbedKey, "new"
		}
		return defaultEmbedKey, "reuse"
	case "new":
		return normalizeSessionKey(key), "new"
	default:
		return normalizeSessionKey(key), normalizeSessionMode(mode, policy)
	}
}

func normalizeSessionKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultEmbedKey
	}
	return trimRunes(value, 191)
}

func appView(app *model.EmbedApp) EmbedAppView {
	if app == nil {
		return EmbedAppView{}
	}
	out := EmbedAppView{
		AppID:          app.AppID,
		Name:           app.Name,
		LauncherTitle:  firstNonEmptyService(app.LauncherTitle, "智能问数"),
		WelcomeMessage: app.WelcomeMessage,
		SessionPolicy:  app.SessionPolicy,
	}
	if app.AllowedOriginsJSON != nil && strings.TrimSpace(*app.AllowedOriginsJSON) != "" {
		_ = json.Unmarshal([]byte(*app.AllowedOriginsJSON), &out.AllowedOrigins)
	}
	return out
}

func datasourceSyncedAt(value *time.Time, fallback string) string {
	if value != nil && !value.IsZero() {
		return value.Format(time.RFC3339)
	}
	return fallback
}

func trimRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
