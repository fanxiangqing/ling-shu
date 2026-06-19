package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	healthService *service.HealthService
}

func NewHealthHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	response.Success(c, h.healthService.Liveness())
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	status := h.healthService.Readiness(c.Request.Context())
	if status.Status != "ok" {
		response.JSON(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "service unavailable", status)
		return
	}
	response.Success(c, status)
}
