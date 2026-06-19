package service

import (
	"context"
	"errors"
	"testing"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestPermissionServiceBindRoleAndCheck(t *testing.T) {
	repo := &permissionFakeRepository{
		roles: []model.Role{
			{BaseModel: model.BaseModel{ID: 1}, Code: "project_admin", Name: "ProjectAdmin", ScopeType: "project"},
		},
		permissions: []model.Permission{
			{ID: 10, Code: "query.execute", Resource: "query", Action: "execute"},
		},
	}
	service := NewPermissionService(repo)

	binding, err := service.BindRole(context.Background(), BindRoleInput{
		UserID:    7,
		RoleCode:  "project_admin",
		TenantID:  1,
		ProjectID: 2,
	})
	if err != nil {
		t.Fatalf("bind role: %v", err)
	}
	if binding.RoleID != 1 || binding.ProjectID != 2 {
		t.Fatalf("unexpected binding: %+v", binding)
	}

	result, err := service.Check(context.Background(), CheckPermissionInput{
		UserID:    7,
		TenantID:  1,
		ProjectID: 2,
		Code:      "query.execute",
	})
	if err != nil {
		t.Fatalf("check permission: %v", err)
	}
	if !result.Allowed || result.Matched == nil || result.Matched.Code != "query.execute" {
		t.Fatalf("unexpected check result: %+v", result)
	}
}

func TestPermissionServiceRejectsInvalidScope(t *testing.T) {
	repo := &permissionFakeRepository{
		roles: []model.Role{{BaseModel: model.BaseModel{ID: 1}, Code: "project_admin", ScopeType: "project"}},
	}
	service := NewPermissionService(repo)

	_, err := service.BindRole(context.Background(), BindRoleInput{UserID: 7, RoleCode: "project_admin", TenantID: 1})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

type permissionFakeRepository struct {
	roles       []model.Role
	permissions []model.Permission
	bindings    []model.RoleBinding
}

func (r *permissionFakeRepository) ListRoles(ctx context.Context) ([]model.Role, error) {
	return r.roles, nil
}

func (r *permissionFakeRepository) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	return r.permissions, nil
}

func (r *permissionFakeRepository) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	for idx := range r.roles {
		if r.roles[idx].Code == code {
			return &r.roles[idx], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *permissionFakeRepository) CreateRoleBinding(ctx context.Context, binding *model.RoleBinding) error {
	binding.ID = uint64(len(r.bindings) + 1)
	r.bindings = append(r.bindings, *binding)
	return nil
}

func (r *permissionFakeRepository) ListRoleBindings(ctx context.Context, filter repository.RoleBindingFilter, page repository.Page) ([]repository.RoleBindingRow, int64, error) {
	rows := r.roleRows(filter)
	return rows, int64(len(rows)), nil
}

func (r *permissionFakeRepository) GetUserRoles(ctx context.Context, filter repository.RoleBindingFilter) ([]repository.RoleBindingRow, error) {
	return r.roleRows(filter), nil
}

func (r *permissionFakeRepository) GetUserPermissions(ctx context.Context, filter repository.RoleBindingFilter) ([]model.Permission, error) {
	if len(r.roleRows(filter)) == 0 {
		return nil, nil
	}
	return r.permissions, nil
}

func (r *permissionFakeRepository) roleRows(filter repository.RoleBindingFilter) []repository.RoleBindingRow {
	var rows []repository.RoleBindingRow
	for _, binding := range r.bindings {
		if filter.UserID > 0 && binding.UserID != filter.UserID {
			continue
		}
		if filter.TenantID > 0 && binding.TenantID != filter.TenantID {
			continue
		}
		if filter.ProjectID > 0 && binding.ProjectID != filter.ProjectID {
			continue
		}
		role := r.roles[0]
		rows = append(rows, repository.RoleBindingRow{
			ID:        binding.ID,
			UserID:    binding.UserID,
			RoleID:    binding.RoleID,
			RoleCode:  role.Code,
			RoleName:  role.Name,
			ScopeType: role.ScopeType,
			TenantID:  binding.TenantID,
			ProjectID: binding.ProjectID,
			CreatedBy: binding.CreatedBy,
		})
	}
	return rows
}
