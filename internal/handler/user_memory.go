package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"ling-shu/internal/memory"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type UserMemoryHandler struct {
	service *service.UserMemoryService
}

type saveUserMemoryRequest struct {
	TenantID   uint64         `json:"tenant_id" binding:"required"`
	Kind       string         `json:"kind"`
	MemoryKey  string         `json:"memory_key"`
	Content    string         `json:"content" binding:"required"`
	Value      map[string]any `json:"value"`
	ScopeLevel string         `json:"scope_level"`
	ExpiresAt  *time.Time     `json:"expires_at"`
}

func NewUserMemoryHandler(userMemoryService *service.UserMemoryService) *UserMemoryHandler {
	return &UserMemoryHandler{service: userMemoryService}
}

func (h *UserMemoryHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.service.List(c.Request.Context(), service.ListUserMemoriesInput{
		TenantID: userMemoryTenantID(c, 0), ProjectID: parseUint64Default(c.Param("project_id"), 0),
		UserID: resolveUserID(c, 0), Statuses: splitCSV(c.Query("status")), Kinds: splitCSV(c.Query("kind")),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserMemoryHandler) Save(c *gin.Context) {
	var req saveUserMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Save(c.Request.Context(), service.SaveUserMemoryInput{
		TenantID: userMemoryTenantID(c, req.TenantID), ProjectID: parseUint64Default(c.Param("project_id"), 0), UserID: resolveUserID(c, 0),
		Kind: req.Kind, Key: req.MemoryKey, Content: req.Content, Value: req.Value, ScopeLevel: req.ScopeLevel, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserMemoryHandler) Update(c *gin.Context) {
	var req saveUserMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Update(c.Request.Context(), service.UpdateUserMemoryInput{
		ID: parseUint64Default(c.Param("id"), 0),
		SaveUserMemoryInput: service.SaveUserMemoryInput{
			TenantID: userMemoryTenantID(c, req.TenantID), ProjectID: parseUint64Default(c.Param("project_id"), 0), UserID: resolveUserID(c, 0),
			Kind: req.Kind, Key: req.MemoryKey, Content: req.Content, Value: req.Value, ScopeLevel: req.ScopeLevel, ExpiresAt: req.ExpiresAt,
		},
	})
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserMemoryHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), userMemoryItemInput(c)); err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *UserMemoryHandler) Confirm(c *gin.Context) {
	item, err := h.service.SetCandidateStatus(c.Request.Context(), userMemoryItemInput(c), true)
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserMemoryHandler) Reject(c *gin.Context) {
	item, err := h.service.SetCandidateStatus(c.Request.Context(), userMemoryItemInput(c), false)
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *UserMemoryHandler) Clear(c *gin.Context) {
	count, err := h.service.Clear(c.Request.Context(), userMemoryItemInput(c), parseBoolDefault(c.Query("include_tenant"), false))
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": count})
}

func (h *UserMemoryHandler) ListEpisodes(c *gin.Context) {
	items, err := h.service.ListEpisodes(c.Request.Context(), userMemoryItemInput(c), int(parseUint64Default(c.Query("limit"), 20)))
	if err != nil {
		writeUserMemoryError(c, err)
		return
	}
	response.Success(c, items)
}

func userMemoryItemInput(c *gin.Context) service.UserMemoryItemInput {
	return service.UserMemoryItemInput{
		TenantID: userMemoryTenantID(c, 0), ProjectID: parseUint64Default(c.Param("project_id"), 0),
		UserID: resolveUserID(c, 0), ID: parseUint64Default(c.Param("id"), 0),
	}
}

func userMemoryTenantID(c *gin.Context, fallback uint64) uint64 {
	return parseUint64Default(c.Param("tenant_id"), parseUint64Default(c.Query("tenant_id"), fallback))
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writeUserMemoryError(c *gin.Context, err error) {
	if errors.Is(err, memory.ErrUserMemoryNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "memory not found")
		return
	}
	writeError(c, err)
}
