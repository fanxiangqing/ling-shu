package service

import (
	"context"
	"errors"
	"testing"
	"time"

	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestAuthServiceCreateAndLogin(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))

	user, err := service.CreateUser(context.Background(), CreateUserInput{
		Username:    "alice",
		Password:    "secret",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.PasswordHash == "secret" || user.ID == 0 {
		t.Fatalf("unexpected user: %+v", user)
	}

	result, err := service.Login(context.Background(), LoginInput{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.AccessToken == "" || result.User.Username != "alice" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if repo.lastLoginUserID != user.ID {
		t.Fatalf("expected last login update for user %d, got %d", user.ID, repo.lastLoginUserID)
	}
}

func TestAuthServiceRejectsWrongPassword(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))
	_, err := service.CreateUser(context.Background(), CreateUserInput{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err = service.Login(context.Background(), LoginInput{Username: "alice", Password: "wrong"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestAuthServiceCreatesMainAccountWorkspace(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour), WithSignupWorkspace("tenant_admin"))

	user, err := service.CreateUser(context.Background(), CreateUserInput{
		Username:    "owner",
		Password:    "secret",
		DisplayName: "Owner",
		TenantName:  "Owner BI",
	})
	if err != nil {
		t.Fatalf("create main account: %v", err)
	}
	if user.ID == 0 || len(repo.tenants) != 1 || repo.tenants[0].Name != "Owner BI" {
		t.Fatalf("workspace was not created: user=%+v tenants=%+v", user, repo.tenants)
	}
	if len(repo.projects) != 0 {
		t.Fatalf("project should be created explicitly by user, got: %+v", repo.projects)
	}
	if len(repo.tenantMembers) != 1 || repo.tenantMembers[0].UserID != user.ID {
		t.Fatalf("tenant member was not created: %+v", repo.tenantMembers)
	}
	if len(repo.roleBindings) != 1 || repo.roleBindings[0].TenantID != repo.tenants[0].ID {
		t.Fatalf("tenant admin role was not bound: %+v", repo.roleBindings)
	}
}

func TestAuthServiceManagesMembers(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))

	tenantMember, err := service.AddTenantMember(context.Background(), AddTenantMemberInput{TenantID: 1, UserID: 7})
	if err != nil {
		t.Fatalf("add tenant member: %v", err)
	}
	if tenantMember.Status != "active" || tenantMember.ID == 0 {
		t.Fatalf("unexpected tenant member: %+v", tenantMember)
	}

	projectMember, err := service.AddProjectMember(context.Background(), AddProjectMemberInput{TenantID: 1, ProjectID: 2, UserID: 7})
	if err != nil {
		t.Fatalf("add project member: %v", err)
	}
	if projectMember.ProjectID != 2 || projectMember.Status != "active" {
		t.Fatalf("unexpected project member: %+v", projectMember)
	}

	tenantMembers, err := service.ListTenantMembers(context.Background(), 1, 1, 20)
	if err != nil {
		t.Fatalf("list tenant members: %v", err)
	}
	if tenantMembers.Total != 1 || tenantMembers.Items[0].UserID != 7 {
		t.Fatalf("unexpected tenant members: %+v", tenantMembers)
	}

	projectMembers, err := service.ListProjectMembers(context.Background(), 1, 2, 1, 20)
	if err != nil {
		t.Fatalf("list project members: %v", err)
	}
	if projectMembers.Total != 1 || projectMembers.Items[0].ProjectID != 2 {
		t.Fatalf("unexpected project members: %+v", projectMembers)
	}
}

type authFakeUserRepository struct {
	users           []model.User
	tenants         []model.Tenant
	projects        []model.Project
	tenantMembers   []model.TenantMember
	projectMembers  []model.ProjectMember
	roleBindings    []model.RoleBinding
	lastLoginUserID uint64
}

func (r *authFakeUserRepository) Create(ctx context.Context, user *model.User) error {
	user.ID = uint64(len(r.users) + 1)
	r.users = append(r.users, *user)
	return nil
}

func (r *authFakeUserRepository) CreateMainAccount(ctx context.Context, user *model.User, tenant *model.Tenant, project *model.Project, roleCode string) error {
	if err := r.Create(ctx, user); err != nil {
		return err
	}
	tenant.ID = uint64(len(r.tenants) + 1)
	if tenant.Code == "" {
		tenant.Code = "org-1"
	}
	r.tenants = append(r.tenants, *tenant)
	member := model.TenantMember{
		BaseModel: model.BaseModel{ID: uint64(len(r.tenantMembers) + 1)},
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Status:    "active",
	}
	r.tenantMembers = append(r.tenantMembers, member)
	if roleCode != "" {
		r.roleBindings = append(r.roleBindings, model.RoleBinding{
			ID:        uint64(len(r.roleBindings) + 1),
			UserID:    user.ID,
			RoleID:    2,
			TenantID:  tenant.ID,
			CreatedBy: user.ID,
		})
	}
	if project != nil {
		project.ID = uint64(len(r.projects) + 1)
		project.TenantID = tenant.ID
		project.CreatedBy = user.ID
		if project.Code == "" {
			project.Code = "default"
		}
		r.projects = append(r.projects, *project)
		r.projectMembers = append(r.projectMembers, model.ProjectMember{
			BaseModel: model.BaseModel{ID: uint64(len(r.projectMembers) + 1)},
			TenantID:  tenant.ID,
			ProjectID: project.ID,
			UserID:    user.ID,
			Status:    "active",
		})
	}
	return nil
}

func (r *authFakeUserRepository) CreateTenantAccount(ctx context.Context, user *model.User, member *model.TenantMember, roleCode string, createdBy uint64) error {
	if err := r.Create(ctx, user); err != nil {
		return err
	}
	member.ID = uint64(len(r.tenantMembers) + 1)
	member.UserID = user.ID
	if member.Status == "" {
		member.Status = "active"
	}
	r.tenantMembers = append(r.tenantMembers, *member)
	if roleCode != "" {
		r.roleBindings = append(r.roleBindings, model.RoleBinding{
			ID:        uint64(len(r.roleBindings) + 1),
			UserID:    user.ID,
			RoleID:    2,
			TenantID:  member.TenantID,
			CreatedBy: createdBy,
		})
	}
	return nil
}

func (r *authFakeUserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	for idx := range r.users {
		if r.users[idx].ID == id {
			return &r.users[idx], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *authFakeUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	for idx := range r.users {
		if r.users[idx].Username == username {
			return &r.users[idx], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *authFakeUserRepository) List(ctx context.Context, page repository.Page) ([]model.User, int64, error) {
	return r.users, int64(len(r.users)), nil
}

func (r *authFakeUserRepository) UpdateLastLogin(ctx context.Context, id uint64, at time.Time) error {
	r.lastLoginUserID = id
	return nil
}

func (r *authFakeUserRepository) AddTenantMember(ctx context.Context, member *model.TenantMember) error {
	member.ID = uint64(len(r.tenantMembers) + 1)
	r.tenantMembers = append(r.tenantMembers, *member)
	return nil
}

func (r *authFakeUserRepository) ListTenantMembers(ctx context.Context, tenantID uint64, page repository.Page) ([]repository.MemberRow, int64, error) {
	var rows []repository.MemberRow
	for _, member := range r.tenantMembers {
		if member.TenantID == tenantID {
			rows = append(rows, repository.MemberRow{
				ID:          member.ID,
				TenantID:    member.TenantID,
				UserID:      member.UserID,
				Username:    "alice",
				DisplayName: "Alice",
				Status:      member.Status,
			})
		}
	}
	return rows, int64(len(rows)), nil
}

func (r *authFakeUserRepository) AddProjectMember(ctx context.Context, member *model.ProjectMember) error {
	member.ID = uint64(len(r.projectMembers) + 1)
	r.projectMembers = append(r.projectMembers, *member)
	return nil
}

func (r *authFakeUserRepository) ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]repository.MemberRow, int64, error) {
	var rows []repository.MemberRow
	for _, member := range r.projectMembers {
		if member.TenantID == tenantID && member.ProjectID == projectID {
			rows = append(rows, repository.MemberRow{
				ID:          member.ID,
				TenantID:    member.TenantID,
				ProjectID:   member.ProjectID,
				UserID:      member.UserID,
				Username:    "alice",
				DisplayName: "Alice",
				Status:      member.Status,
			})
		}
	}
	return rows, int64(len(rows)), nil
}
