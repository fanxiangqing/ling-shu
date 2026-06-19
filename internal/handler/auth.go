package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

type createUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
	TenantName  string `json:"tenant_name"`
	TenantCode  string `json:"tenant_code"`
	ProjectName string `json:"project_name"`
	ProjectCode string `json:"project_code"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type addMemberRequest struct {
	TenantID uint64 `json:"tenant_id"`
	UserID   uint64 `json:"user_id" binding:"required"`
}

type updateMemberStatusRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Status   string `json:"status" binding:"required"`
}

type createTenantUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
	RoleCode    string `json:"role_code"`
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	user, err := h.authService.CreateUser(c.Request.Context(), service.CreateUserInput{
		Username:    req.Username,
		Email:       req.Email,
		Mobile:      req.Mobile,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		TenantName:  req.TenantName,
		TenantCode:  req.TenantCode,
		ProjectName: req.ProjectName,
		ProjectCode: req.ProjectCode,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.authService.Login(c.Request.Context(), service.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) CreateTenantUser(c *gin.Context) {
	var req createTenantUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	user, err := h.authService.CreateTenantUser(c.Request.Context(), service.CreateTenantUserInput{
		TenantID:    parseUint64Default(c.Param("tenant_id"), 0),
		Username:    req.Username,
		Email:       req.Email,
		Mobile:      req.Mobile,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		RoleCode:    req.RoleCode,
		CreatedBy:   authenticatedUserID(c),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) AddTenantMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	tenantID := parseUint64Default(c.Param("tenant_id"), req.TenantID)
	member, err := h.authService.AddTenantMember(c.Request.Context(), service.AddTenantMemberInput{
		TenantID: tenantID,
		UserID:   req.UserID,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, member)
}

func (h *AuthHandler) ListTenantMembers(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.authService.ListTenantMembers(
		c.Request.Context(),
		parseUint64Default(c.Param("tenant_id"), parseUint64Default(c.Query("tenant_id"), 0)),
		page,
		pageSize,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) UpdateTenantMemberStatus(c *gin.Context) {
	var req updateMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	err := h.authService.UpdateTenantMemberStatus(c.Request.Context(), service.UpdateTenantMemberStatusInput{
		TenantID: parseUint64Default(c.Param("tenant_id"), req.TenantID),
		MemberID: parseUint64Default(c.Param("member_id"), 0),
		Status:   req.Status,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "updated"})
}

func (h *AuthHandler) DeleteTenantMember(c *gin.Context) {
	err := h.authService.DeleteTenantMember(c.Request.Context(), service.DeleteTenantMemberInput{
		TenantID: parseUint64Default(c.Param("tenant_id"), 0),
		MemberID: parseUint64Default(c.Param("member_id"), 0),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "deleted"})
}

func (h *AuthHandler) AddProjectMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	member, err := h.authService.AddProjectMember(c.Request.Context(), service.AddProjectMemberInput{
		TenantID:  req.TenantID,
		ProjectID: parseUint64Default(c.Param("project_id"), 0),
		UserID:    req.UserID,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, member)
}

func (h *AuthHandler) ListProjectMembers(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.authService.ListProjectMembers(
		c.Request.Context(),
		parseUint64Default(c.Query("tenant_id"), 0),
		parseUint64Default(c.Param("project_id"), 0),
		page,
		pageSize,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) UpdateProjectMemberStatus(c *gin.Context) {
	var req updateMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	err := h.authService.UpdateProjectMemberStatus(c.Request.Context(), service.UpdateProjectMemberStatusInput{
		TenantID:  parseUint64Default(c.Query("tenant_id"), req.TenantID),
		ProjectID: parseUint64Default(c.Param("project_id"), 0),
		MemberID:  parseUint64Default(c.Param("member_id"), 0),
		Status:    req.Status,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "updated"})
}

func (h *AuthHandler) DeleteProjectMember(c *gin.Context) {
	err := h.authService.DeleteProjectMember(c.Request.Context(), service.DeleteProjectMemberInput{
		TenantID:  parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID: parseUint64Default(c.Param("project_id"), 0),
		MemberID:  parseUint64Default(c.Param("member_id"), 0),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "deleted"})
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.authService.ListUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
