package service

import (
	"context"
	"errors"
	"testing"
	"time"

	authpkg "ling-shu/internal/auth"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"gorm.io/gorm"
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
	repo.tenantMembers = append(repo.tenantMembers, model.TenantMember{
		BaseModel: model.BaseModel{ID: 1},
		TenantID:  1,
		UserID:    user.ID,
		Status:    "active",
	})

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
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestAuthServiceRejectsUnknownLoginAccount(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))

	_, err := service.Login(context.Background(), LoginInput{Username: "missing", Password: "secret"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestAuthServiceRejectsLoginWithoutActiveWorkspace(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))
	user, err := service.CreateUser(context.Background(), CreateUserInput{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	repo.tenantMembers = append(repo.tenantMembers, model.TenantMember{
		BaseModel: model.BaseModel{ID: 1, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
		TenantID:  1,
		UserID:    user.ID,
		Status:    "active",
	})

	_, err = service.Login(context.Background(), LoginInput{Username: "alice", Password: "secret"})
	if !errors.Is(err, ErrNoActiveWorkspace) {
		t.Fatalf("expected no active workspace, got %v", err)
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

	if err := service.UpdateTenantMemberStatus(context.Background(), UpdateTenantMemberStatusInput{TenantID: 1, MemberID: tenantMember.ID, Status: "inactive"}); err != nil {
		t.Fatalf("update tenant member status: %v", err)
	}
	if repo.tenantMembers[0].Status != "inactive" {
		t.Fatalf("tenant member was not inactive: %+v", repo.tenantMembers[0])
	}

	if err := service.UpdateProjectMemberStatus(context.Background(), UpdateProjectMemberStatusInput{TenantID: 1, ProjectID: 2, MemberID: projectMember.ID, Status: "inactive"}); err != nil {
		t.Fatalf("update project member status: %v", err)
	}
	if repo.projectMembers[0].Status != "inactive" {
		t.Fatalf("project member was not inactive: %+v", repo.projectMembers[0])
	}

	if err := service.DeleteTenantMember(context.Background(), DeleteTenantMemberInput{TenantID: 1, MemberID: tenantMember.ID}); err != nil {
		t.Fatalf("delete tenant member: %v", err)
	}
	tenantMembers, err = service.ListTenantMembers(context.Background(), 1, 1, 20)
	if err != nil {
		t.Fatalf("list tenant members after delete: %v", err)
	}
	if tenantMembers.Total != 0 {
		t.Fatalf("expected tenant member to be deleted, got %+v", tenantMembers)
	}
	projectMembers, err = service.ListProjectMembers(context.Background(), 1, 2, 1, 20)
	if err != nil {
		t.Fatalf("list project members after tenant delete: %v", err)
	}
	if projectMembers.Total != 0 {
		t.Fatalf("expected project member to be deleted with tenant member, got %+v", projectMembers)
	}
}

func TestAuthServiceListMembersFiltersEmbedSubjects(t *testing.T) {
	repo := &authFakeUserRepository{
		users: []model.User{
			{
				BaseModel:    model.BaseModel{ID: 7},
				Username:     "analyst",
				DisplayName:  "数据分析师",
				PasswordHash: "hashed-password",
				Status:       "active",
			},
			{
				BaseModel:    model.BaseModel{ID: 8},
				Username:     "embed_emb_app_abc",
				DisplayName:  "三方系统测试用户",
				PasswordHash: "embed-subject:abcdef",
				Status:       "active",
			},
		},
		tenantMembers: []model.TenantMember{
			{BaseModel: model.BaseModel{ID: 1}, TenantID: 1, UserID: 7, Status: "active"},
			{BaseModel: model.BaseModel{ID: 2}, TenantID: 1, UserID: 8, Status: "active"},
		},
		projectMembers: []model.ProjectMember{
			{BaseModel: model.BaseModel{ID: 1}, TenantID: 1, ProjectID: 2, UserID: 7, Status: "active"},
			{BaseModel: model.BaseModel{ID: 2}, TenantID: 1, ProjectID: 2, UserID: 8, Status: "active"},
		},
	}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour))

	tenantMembers, err := service.ListTenantMembers(context.Background(), 1, 1, 20)
	if err != nil {
		t.Fatalf("list tenant members: %v", err)
	}
	if tenantMembers.Total != 1 || tenantMembers.Items[0].UserID != 7 {
		t.Fatalf("expected only regular tenant member, got %+v", tenantMembers)
	}

	projectMembers, err := service.ListProjectMembers(context.Background(), 1, 2, 1, 20)
	if err != nil {
		t.Fatalf("list project members: %v", err)
	}
	if projectMembers.Total != 1 || projectMembers.Items[0].UserID != 7 {
		t.Fatalf("expected only regular project member, got %+v", projectMembers)
	}
}

func TestAuthServiceProtectsPrimaryAdminMember(t *testing.T) {
	repo := &authFakeUserRepository{}
	service := NewAuthService(repo, authpkg.NewTokenManager("secret", time.Hour), WithSignupWorkspace("tenant_admin"))
	user, err := service.CreateUser(context.Background(), CreateUserInput{Username: "owner", Password: "secret"})
	if err != nil {
		t.Fatalf("create main account: %v", err)
	}
	repo.projectMembers = append(repo.projectMembers, model.ProjectMember{
		BaseModel: model.BaseModel{ID: 1},
		TenantID:  repo.tenants[0].ID,
		ProjectID: 9,
		UserID:    user.ID,
		Status:    "active",
	})

	err = service.UpdateTenantMemberStatus(context.Background(), UpdateTenantMemberStatusInput{
		TenantID: repo.tenants[0].ID,
		MemberID: repo.tenantMembers[0].ID,
		Status:   "inactive",
	})
	if !errors.Is(err, ErrPrimaryAdminLocked) {
		t.Fatalf("expected primary admin lock, got %v", err)
	}

	err = service.DeleteProjectMember(context.Background(), DeleteProjectMemberInput{
		TenantID:  repo.tenants[0].ID,
		ProjectID: 9,
		MemberID:  1,
	})
	if !errors.Is(err, ErrPrimaryAdminLocked) {
		t.Fatalf("expected primary admin lock, got %v", err)
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
	return nil, gorm.ErrRecordNotFound
}

func (r *authFakeUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	for idx := range r.users {
		if r.users[idx].Username == username {
			return &r.users[idx], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *authFakeUserRepository) HasActiveWorkspace(ctx context.Context, userID uint64) (bool, error) {
	for _, member := range r.tenantMembers {
		if member.UserID == userID && member.Status == "active" && !member.DeletedAt.Valid {
			return true, nil
		}
	}
	for _, binding := range r.roleBindings {
		if binding.UserID == userID && binding.RoleID == 1 {
			return true, nil
		}
	}
	return false, nil
}

func (r *authFakeUserRepository) List(ctx context.Context, page repository.Page) ([]model.User, int64, error) {
	return r.users, int64(len(r.users)), nil
}

func (r *authFakeUserRepository) UpdateLastLogin(ctx context.Context, id uint64, at time.Time) error {
	r.lastLoginUserID = id
	return nil
}

func (r *authFakeUserRepository) AddTenantMember(ctx context.Context, member *model.TenantMember) error {
	for idx := range r.tenantMembers {
		existing := &r.tenantMembers[idx]
		if existing.TenantID == member.TenantID && existing.UserID == member.UserID {
			existing.Status = member.Status
			existing.DeletedAt = gorm.DeletedAt{}
			*member = *existing
			return nil
		}
	}
	member.ID = uint64(len(r.tenantMembers) + 1)
	r.tenantMembers = append(r.tenantMembers, *member)
	return nil
}

func (r *authFakeUserRepository) ListTenantMembers(ctx context.Context, tenantID uint64, page repository.Page) ([]repository.MemberRow, int64, error) {
	var rows []repository.MemberRow
	for _, member := range r.tenantMembers {
		if member.TenantID == tenantID && !member.DeletedAt.Valid {
			user := r.fakeUser(member.UserID)
			if user != nil && repository.IsEmbedSubjectPasswordHash(user.PasswordHash) {
				continue
			}
			rows = append(rows, repository.MemberRow{
				ID:          member.ID,
				TenantID:    member.TenantID,
				UserID:      member.UserID,
				Username:    fakeMemberUsername(user),
				DisplayName: fakeMemberDisplayName(user),
				Status:      member.Status,
			})
		}
	}
	return rows, int64(len(rows)), nil
}

func (r *authFakeUserRepository) IsTenantPrimaryAdminMember(ctx context.Context, tenantID uint64, memberID uint64) (bool, error) {
	for _, member := range r.tenantMembers {
		if member.TenantID != tenantID || member.ID != memberID {
			continue
		}
		for _, binding := range r.roleBindings {
			if binding.UserID == member.UserID && binding.TenantID == tenantID && binding.CreatedBy == member.UserID && binding.RoleID == 2 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *authFakeUserRepository) UpdateTenantMemberStatus(ctx context.Context, tenantID uint64, memberID uint64, status string) error {
	for idx := range r.tenantMembers {
		if r.tenantMembers[idx].TenantID == tenantID && r.tenantMembers[idx].ID == memberID && !r.tenantMembers[idx].DeletedAt.Valid {
			r.tenantMembers[idx].Status = status
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *authFakeUserRepository) DeleteTenantMember(ctx context.Context, tenantID uint64, memberID uint64) error {
	for idx := range r.tenantMembers {
		if r.tenantMembers[idx].TenantID == tenantID && r.tenantMembers[idx].ID == memberID && !r.tenantMembers[idx].DeletedAt.Valid {
			r.tenantMembers[idx].DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
			userID := r.tenantMembers[idx].UserID
			for projectIdx := range r.projectMembers {
				if r.projectMembers[projectIdx].TenantID == tenantID && r.projectMembers[projectIdx].UserID == userID {
					r.projectMembers[projectIdx].DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
				}
			}
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *authFakeUserRepository) AddProjectMember(ctx context.Context, member *model.ProjectMember) error {
	for idx := range r.projectMembers {
		existing := &r.projectMembers[idx]
		if existing.TenantID == member.TenantID && existing.ProjectID == member.ProjectID && existing.UserID == member.UserID {
			existing.Status = member.Status
			existing.DeletedAt = gorm.DeletedAt{}
			*member = *existing
			return nil
		}
	}
	member.ID = uint64(len(r.projectMembers) + 1)
	r.projectMembers = append(r.projectMembers, *member)
	return nil
}

func (r *authFakeUserRepository) ListProjectMembers(ctx context.Context, tenantID uint64, projectID uint64, page repository.Page) ([]repository.MemberRow, int64, error) {
	var rows []repository.MemberRow
	for _, member := range r.projectMembers {
		if member.TenantID == tenantID && member.ProjectID == projectID && !member.DeletedAt.Valid {
			user := r.fakeUser(member.UserID)
			if user != nil && repository.IsEmbedSubjectPasswordHash(user.PasswordHash) {
				continue
			}
			rows = append(rows, repository.MemberRow{
				ID:          member.ID,
				TenantID:    member.TenantID,
				ProjectID:   member.ProjectID,
				UserID:      member.UserID,
				Username:    fakeMemberUsername(user),
				DisplayName: fakeMemberDisplayName(user),
				Status:      member.Status,
			})
		}
	}
	return rows, int64(len(rows)), nil
}

func (r *authFakeUserRepository) fakeUser(userID uint64) *model.User {
	for idx := range r.users {
		if r.users[idx].ID == userID {
			return &r.users[idx]
		}
	}
	return nil
}

func fakeMemberUsername(user *model.User) string {
	if user == nil || user.Username == "" {
		return "alice"
	}
	return user.Username
}

func fakeMemberDisplayName(user *model.User) string {
	if user == nil || user.DisplayName == "" {
		return "Alice"
	}
	return user.DisplayName
}

func (r *authFakeUserRepository) IsProjectPrimaryAdminMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) (bool, error) {
	for _, member := range r.projectMembers {
		if member.TenantID != tenantID || member.ProjectID != projectID || member.ID != memberID {
			continue
		}
		for _, binding := range r.roleBindings {
			if binding.UserID == member.UserID && binding.TenantID == tenantID && binding.CreatedBy == member.UserID && binding.RoleID == 2 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *authFakeUserRepository) UpdateProjectMemberStatus(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64, status string) error {
	for idx := range r.projectMembers {
		if r.projectMembers[idx].TenantID == tenantID && r.projectMembers[idx].ProjectID == projectID && r.projectMembers[idx].ID == memberID && !r.projectMembers[idx].DeletedAt.Valid {
			r.projectMembers[idx].Status = status
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *authFakeUserRepository) DeleteProjectMember(ctx context.Context, tenantID uint64, projectID uint64, memberID uint64) error {
	for idx := range r.projectMembers {
		if r.projectMembers[idx].TenantID == tenantID && r.projectMembers[idx].ProjectID == projectID && r.projectMembers[idx].ID == memberID && !r.projectMembers[idx].DeletedAt.Valid {
			r.projectMembers[idx].DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
