package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
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

type UpdateTenantMemberStatusInput struct {
	TenantID uint64
	MemberID uint64
	Status   string
}

type DeleteTenantMemberInput struct {
	TenantID uint64
	MemberID uint64
}

type UpdateProjectMemberStatusInput struct {
	TenantID  uint64
	ProjectID uint64
	MemberID  uint64
	Status    string
}

type DeleteProjectMemberInput struct {
	TenantID  uint64
	ProjectID uint64
	MemberID  uint64
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Status != "active" {
		s.logger.Warn("login rejected",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.String("status", user.Status),
		)
		return nil, ErrUserDisabled
	}
	if !authpkg.CheckPassword(user.PasswordHash, password) {
		s.logger.Warn("login rejected",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.String("status", user.Status),
		)
		return nil, ErrInvalidCredentials
	}
	hasWorkspace, err := s.userRepo.HasActiveWorkspace(ctx, user.ID)
	if err != nil {
		s.logger.Error("login workspace lookup failed",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
			zap.Error(err),
		)
		return nil, err
	}
	if !hasWorkspace {
		s.logger.Warn("login rejected without active workspace",
			zap.Uint64("user_id", user.ID),
			zap.String("username_hash", sqlHash(username)),
		)
		return nil, ErrNoActiveWorkspace
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

func (s *AuthService) UpdateTenantMemberStatus(ctx context.Context, input UpdateTenantMemberStatusInput) error {
	status := normalizeMemberStatus(input.Status)
	if input.TenantID == 0 || input.MemberID == 0 || status == "" {
		return ErrInvalidInput
	}
	protected, err := s.userRepo.IsTenantPrimaryAdminMember(ctx, input.TenantID, input.MemberID)
	if err != nil {
		s.logger.Error("tenant member primary admin lookup failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	if protected {
		return ErrPrimaryAdminLocked
	}
	if err := s.userRepo.UpdateTenantMemberStatus(ctx, input.TenantID, input.MemberID, status); err != nil {
		s.logger.Error("tenant member status update failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("member_id", input.MemberID),
			zap.String("status", status),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("tenant member status updated",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("member_id", input.MemberID),
		zap.String("status", status),
	)
	return nil
}

func (s *AuthService) DeleteTenantMember(ctx context.Context, input DeleteTenantMemberInput) error {
	if input.TenantID == 0 || input.MemberID == 0 {
		return ErrInvalidInput
	}
	protected, err := s.userRepo.IsTenantPrimaryAdminMember(ctx, input.TenantID, input.MemberID)
	if err != nil {
		s.logger.Error("tenant member primary admin lookup failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	if protected {
		return ErrPrimaryAdminLocked
	}
	if err := s.userRepo.DeleteTenantMember(ctx, input.TenantID, input.MemberID); err != nil {
		s.logger.Error("tenant member delete failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("tenant member deleted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("member_id", input.MemberID),
	)
	return nil
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

func (s *AuthService) UpdateProjectMemberStatus(ctx context.Context, input UpdateProjectMemberStatusInput) error {
	status := normalizeMemberStatus(input.Status)
	if input.TenantID == 0 || input.ProjectID == 0 || input.MemberID == 0 || status == "" {
		return ErrInvalidInput
	}
	protected, err := s.userRepo.IsProjectPrimaryAdminMember(ctx, input.TenantID, input.ProjectID, input.MemberID)
	if err != nil {
		s.logger.Error("project member primary admin lookup failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	if protected {
		return ErrPrimaryAdminLocked
	}
	if err := s.userRepo.UpdateProjectMemberStatus(ctx, input.TenantID, input.ProjectID, input.MemberID, status); err != nil {
		s.logger.Error("project member status update failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("member_id", input.MemberID),
			zap.String("status", status),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("project member status updated",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("member_id", input.MemberID),
		zap.String("status", status),
	)
	return nil
}

func (s *AuthService) DeleteProjectMember(ctx context.Context, input DeleteProjectMemberInput) error {
	if input.TenantID == 0 || input.ProjectID == 0 || input.MemberID == 0 {
		return ErrInvalidInput
	}
	protected, err := s.userRepo.IsProjectPrimaryAdminMember(ctx, input.TenantID, input.ProjectID, input.MemberID)
	if err != nil {
		s.logger.Error("project member primary admin lookup failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	if protected {
		return ErrPrimaryAdminLocked
	}
	if err := s.userRepo.DeleteProjectMember(ctx, input.TenantID, input.ProjectID, input.MemberID); err != nil {
		s.logger.Error("project member delete failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("member_id", input.MemberID),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("project member deleted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("member_id", input.MemberID),
	)
	return nil
}

func normalizeMemberStatus(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "active":
		return "active"
	case "inactive", "disabled", "paused":
		return "inactive"
	default:
		return ""
	}
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
