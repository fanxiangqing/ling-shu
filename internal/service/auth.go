package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

type AuthService struct {
	userRepo              repository.UserRepository
	tokens                *authpkg.TokenManager
	createWorkspaceSignup bool
	mainRoleCode          string
	logger                *zap.Logger
}

type CreateUserInput struct {
	Username    string
	Email       string
	Mobile      string
	Password    string
	DisplayName string
	TenantName  string
	TenantCode  string
	ProjectName string
	ProjectCode string
}

type CreateTenantUserInput struct {
	TenantID    uint64
	Username    string
	Email       string
	Mobile      string
	Password    string
	DisplayName string
	RoleCode    string
	CreatedBy   uint64
}

type LoginInput struct {
	Username string
	Password string
}

type LoginResult struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	User        *model.User `json:"user"`
}

type AddTenantMemberInput struct {
	TenantID uint64
	UserID   uint64
}

type AddProjectMemberInput struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
}

type AuthOption func(*AuthService)

func WithSignupWorkspace(roleCode string) AuthOption {
	return func(s *AuthService) {
		s.createWorkspaceSignup = true
		s.mainRoleCode = strings.TrimSpace(roleCode)
	}
}

func NewAuthService(userRepo repository.UserRepository, tokens *authpkg.TokenManager, options ...AuthOption) *AuthService {
	service := &AuthService{userRepo: userRepo, tokens: tokens, logger: zap.NewNop()}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *AuthService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *AuthService) CreateUser(ctx context.Context, input CreateUserInput) (*model.User, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	displayName := strings.TrimSpace(input.DisplayName)
	if username == "" || password == "" {
		return nil, ErrInvalidInput
	}
	if displayName == "" {
		displayName = username
	}
	passwordHash, err := authpkg.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:     username,
		Email:        optionalString(input.Email),
		Mobile:       optionalString(input.Mobile),
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		Status:       "active",
	}
	if s.createWorkspaceSignup {
		tenant := &model.Tenant{
			Name:   signupTenantName(input.TenantName, displayName),
			Code:   normalizeCode(input.TenantCode),
			Status: "active",
		}
		if err := s.userRepo.CreateMainAccount(ctx, user, tenant, nil, s.mainRoleCode); err != nil {
			s.logger.Error("main account create failed",
				zap.String("username_hash", sqlHash(username)),
				zap.String("tenant_code", tenant.Code),
				zap.String("role_code", s.mainRoleCode),
				zap.Error(err),
			)
			return nil, err
		}
		s.logger.Info("main account created",
			zap.Uint64("user_id", user.ID),
			zap.Uint64("tenant_id", tenant.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.String("tenant_code", tenant.Code),
			zap.String("role_code", s.mainRoleCode),
		)
		return user, nil
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("user create failed",
			zap.String("username_hash", sqlHash(username)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("user created",
		zap.Uint64("user_id", user.ID),
		zap.String("username_hash", sqlHash(username)),
	)
	return user, nil
}

func (s *AuthService) CreateTenantUser(ctx context.Context, input CreateTenantUserInput) (*model.User, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	displayName := strings.TrimSpace(input.DisplayName)
	roleCode := strings.TrimSpace(input.RoleCode)
	if input.TenantID == 0 || username == "" || password == "" {
		return nil, ErrInvalidInput
	}
	if roleCode != "" && roleCode != "tenant_admin" {
		return nil, ErrInvalidInput
	}
	if displayName == "" {
		displayName = username
	}
	passwordHash, err := authpkg.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:     username,
		Email:        optionalString(input.Email),
		Mobile:       optionalString(input.Mobile),
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		Status:       "active",
	}
	member := &model.TenantMember{
		TenantID: input.TenantID,
		Status:   "active",
	}
	if err := s.userRepo.CreateTenantAccount(ctx, user, member, roleCode, input.CreatedBy); err != nil {
		s.logger.Error("tenant account create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("created_by", input.CreatedBy),
			zap.String("username_hash", sqlHash(username)),
			zap.String("role_code", roleCode),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("tenant account created",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("user_id", user.ID),
		zap.Uint64("created_by", input.CreatedBy),
		zap.String("username_hash", sqlHash(username)),
		zap.String("role_code", roleCode),
	)
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" || s.tokens == nil {
		return nil, ErrInvalidInput
	}
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		s.logger.Warn("login user lookup failed",
			zap.String("username_hash", sqlHash(username)),
			zap.Error(err),
		)
		return nil, err
	}
	if user.Status != "active" || !authpkg.CheckPassword(user.PasswordHash, password) {
		s.logger.Warn("login rejected",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.String("status", user.Status),
		)
		return nil, ErrInvalidInput
	}
	token, expiresAt, err := s.tokens.Generate(user.ID, user.Username)
	if err != nil {
		s.logger.Error("login token generate failed",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.Error(err),
		)
		return nil, err
	}
	now := time.Now()
	user.LastLoginAt = &now
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID, now)
	s.logger.Info("login succeeded",
		zap.Uint64("user_id", user.ID),
		zap.String("username_hash", sqlHash(username)),
		zap.Time("expires_at", expiresAt),
	)
	return &LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User:        user,
	}, nil
}

func (s *AuthService) ListUsers(ctx context.Context, page int, pageSize int) (PageResult[model.User], error) {
	p := NewPage(page, pageSize)
	items, total, err := s.userRepo.List(ctx, p)
	if err != nil {
		s.logger.Error("user list failed",
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.User]{}, err
	}
	return PageResult[model.User]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *AuthService) AddTenantMember(ctx context.Context, input AddTenantMemberInput) (*model.TenantMember, error) {
	if input.TenantID == 0 || input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	member := &model.TenantMember{
		TenantID: input.TenantID,
		UserID:   input.UserID,
		Status:   "active",
	}
	if err := s.userRepo.AddTenantMember(ctx, member); err != nil {
		s.logger.Error("tenant member add failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("tenant member added",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("member_id", member.ID),
	)
	return member, nil
}

func (s *AuthService) ListTenantMembers(ctx context.Context, tenantID uint64, page int, pageSize int) (PageResult[repository.MemberRow], error) {
	if tenantID == 0 {
		return PageResult[repository.MemberRow]{}, ErrInvalidInput
	}
	p := NewPage(page, pageSize)
	items, total, err := s.userRepo.ListTenantMembers(ctx, tenantID, p)
	if err != nil {
		s.logger.Error("tenant member list failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[repository.MemberRow]{}, err
	}
	return PageResult[repository.MemberRow]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *AuthService) AddProjectMember(ctx context.Context, input AddProjectMemberInput) (*model.ProjectMember, error) {
	if input.TenantID == 0 || input.ProjectID == 0 || input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	member := &model.ProjectMember{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		UserID:    input.UserID,
		Status:    "active",
	}
	if err := s.userRepo.AddProjectMember(ctx, member); err != nil {
		s.logger.Error("project member add failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("project member added",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("member_id", member.ID),
	)
	return member, nil
}

func (s *AuthService) ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page int, pageSize int) (PageResult[repository.MemberRow], error) {
	if tenantID == 0 || projectID == 0 {
		return PageResult[repository.MemberRow]{}, ErrInvalidInput
	}
	p := NewPage(page, pageSize)
	items, total, err := s.userRepo.ListProjectMembers(ctx, tenantID, projectID, p)
	if err != nil {
		s.logger.Error("project member list failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[repository.MemberRow]{}, err
	}
	return PageResult[repository.MemberRow]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func signupTenantName(input string, displayName string) string {
	name := strings.TrimSpace(input)
	if name != "" {
		return name
	}
	if strings.TrimSpace(displayName) == "" {
		return "默认组织"
	}
	return fmt.Sprintf("%s的组织", displayName)
}

func normalizeCode(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r)
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func optionalString(input string) *string {
	value := strings.TrimSpace(input)
	if value == "" {
		return nil
	}
	return &value
}
