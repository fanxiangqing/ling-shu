package service

import (
	"context"
	"strings"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

type PermissionService struct {
	permissionRepo repository.PermissionRepository
	logger         *zap.Logger
}

type BindRoleInput struct {
	UserID    uint64
	RoleCode  string
	TenantID  uint64
	ProjectID uint64
	CreatedBy uint64
}

type ListRoleBindingsInput struct {
	UserID    uint64
	TenantID  uint64
	ProjectID uint64
	Page      int
	PageSize  int
}

type CheckPermissionInput struct {
	UserID    uint64
	TenantID  uint64
	ProjectID uint64
	Code      string
	Resource  string
	Action    string
}

type PermissionCheckResult struct {
	Allowed     bool                        `json:"allowed"`
	Roles       []repository.RoleBindingRow `json:"roles"`
	Permissions []model.Permission          `json:"permissions"`
	Matched     *model.Permission           `json:"matched,omitempty"`
}

func NewPermissionService(permissionRepo repository.PermissionRepository) *PermissionService {
	return &PermissionService{permissionRepo: permissionRepo, logger: zap.NewNop()}
}

func (s *PermissionService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *PermissionService) ListRoles(ctx context.Context) ([]model.Role, error) {
	roles, err := s.permissionRepo.ListRoles(ctx)
	if err != nil {
		s.logger.Error("role list failed", zap.Error(err))
		return nil, err
	}
	return roles, nil
}

func (s *PermissionService) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	permissions, err := s.permissionRepo.ListPermissions(ctx)
	if err != nil {
		s.logger.Error("permission list failed", zap.Error(err))
		return nil, err
	}
	return permissions, nil
}

func (s *PermissionService) BindRole(ctx context.Context, input BindRoleInput) (*model.RoleBinding, error) {
	roleCode := strings.TrimSpace(input.RoleCode)
	if input.UserID == 0 || roleCode == "" {
		return nil, ErrInvalidInput
	}
	role, err := s.permissionRepo.GetRoleByCode(ctx, roleCode)
	if err != nil {
		s.logger.Error("role binding role lookup failed",
			zap.Uint64("user_id", input.UserID),
			zap.String("role_code", roleCode),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}
	if !validRoleScope(role.ScopeType, input.TenantID, input.ProjectID) {
		s.logger.Warn("role binding scope rejected",
			zap.Uint64("user_id", input.UserID),
			zap.String("role_code", roleCode),
			zap.String("scope_type", role.ScopeType),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
		)
		return nil, ErrInvalidInput
	}
	binding := &model.RoleBinding{
		UserID:    input.UserID,
		RoleID:    role.ID,
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		CreatedBy: input.CreatedBy,
	}
	if err := s.permissionRepo.CreateRoleBinding(ctx, binding); err != nil {
		s.logger.Error("role binding create failed",
			zap.Uint64("user_id", input.UserID),
			zap.Uint64("role_id", role.ID),
			zap.String("role_code", roleCode),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("created_by", input.CreatedBy),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("role binding created",
		zap.Uint64("binding_id", binding.ID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("role_id", role.ID),
		zap.String("role_code", roleCode),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("created_by", input.CreatedBy),
	)
	return binding, nil
}

func (s *PermissionService) ListRoleBindings(ctx context.Context, input ListRoleBindingsInput) (PageResult[repository.RoleBindingRow], error) {
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.permissionRepo.ListRoleBindings(ctx, repository.RoleBindingFilter{
		UserID:    input.UserID,
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
	}, p)
	if err != nil {
		s.logger.Error("role binding list failed",
			zap.Uint64("user_id", input.UserID),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[repository.RoleBindingRow]{}, err
	}
	return PageResult[repository.RoleBindingRow]{Items: items, Total: total, Page: p.Page, PageSize: p.Limit()}, nil
}

func (s *PermissionService) Check(ctx context.Context, input CheckPermissionInput) (*PermissionCheckResult, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	code := strings.TrimSpace(input.Code)
	resource := strings.TrimSpace(input.Resource)
	action := strings.TrimSpace(input.Action)
	if code == "" && (resource == "" || action == "") {
		return nil, ErrInvalidInput
	}
	filter := repository.RoleBindingFilter{
		UserID:    input.UserID,
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
	}
	roles, err := s.permissionRepo.GetUserRoles(ctx, filter)
	if err != nil {
		s.logger.Error("permission check role lookup failed",
			zap.Uint64("user_id", input.UserID),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("code", code),
			zap.String("resource", resource),
			zap.String("action", action),
			zap.Error(err),
		)
		return nil, err
	}
	permissions, err := s.permissionRepo.GetUserPermissions(ctx, filter)
	if err != nil {
		s.logger.Error("permission check permission lookup failed",
			zap.Uint64("user_id", input.UserID),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("code", code),
			zap.String("resource", resource),
			zap.String("action", action),
			zap.Error(err),
		)
		return nil, err
	}
	result := &PermissionCheckResult{
		Roles:       roles,
		Permissions: permissions,
	}
	for idx := range permissions {
		permission := permissions[idx]
		if permissionMatches(permission, code, resource, action) {
			result.Allowed = true
			result.Matched = &permission
			break
		}
	}
	return result, nil
}

func validRoleScope(scopeType string, tenantID uint64, projectID uint64) bool {
	switch scopeType {
	case "global":
		return tenantID == 0 && projectID == 0
	case "tenant":
		return tenantID > 0 && projectID == 0
	case "project":
		return tenantID > 0 && projectID > 0
	default:
		return false
	}
}

func permissionMatches(permission model.Permission, code string, resource string, action string) bool {
	if code != "" {
		return permission.Code == code
	}
	return permission.Resource == resource && permission.Action == action
}
