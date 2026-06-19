package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type TenantHandler struct {
	tenantService *service.TenantService
}

type createTenantRequest struct {
	Name string `json:"name" binding:"required"`
	Code string `json:"code" binding:"required"`
}

func NewTenantHandler(tenantService *service.TenantService) *TenantHandler {
	return &TenantHandler{tenantService: tenantService}
}

func (h *TenantHandler) Create(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	tenant, err := h.tenantService.Create(c.Request.Context(), service.CreateTenantInput{
		Name: req.Name,
		Code: req.Code,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, tenant)
}

func (h *TenantHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.tenantService.List(c.Request.Context(), page, pageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
