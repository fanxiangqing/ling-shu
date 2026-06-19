package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type DatasourceHandler struct {
	datasourceService *service.DatasourceService
}

type createDatasourceRequest struct {
	TenantID   uint64 `json:"tenant_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	DBType     string `json:"db_type" binding:"required"`
	DSN        string `json:"dsn" binding:"required"`
	ConfigJSON string `json:"config_json"`
	CreatedBy  uint64 `json:"created_by"`
}

type syncDatasourceRequest struct {
	TriggerType string `json:"trigger_type"`
	UserID      uint64 `json:"user_id"`
}

type testDatasourceConnectionRequest struct {
	TenantID   uint64 `json:"tenant_id"`
	DBType     string `json:"db_type" binding:"required"`
	DSN        string `json:"dsn" binding:"required"`
	ConfigJSON string `json:"config_json"`
}

type updateMetadataCommentRequest struct {
	Comment string `json:"comment"`
	UserID  uint64 `json:"user_id"`
}

func NewDatasourceHandler(datasourceService *service.DatasourceService) *DatasourceHandler {
	return &DatasourceHandler{datasourceService: datasourceService}
}

func (h *DatasourceHandler) Create(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	tenantID := parseUint64Default(c.Param("tenant_id"), 0)
	var req createDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if tenantID == 0 {
		tenantID = req.TenantID
	}

	datasource, err := h.datasourceService.Create(c.Request.Context(), service.CreateDatasourceInput{
		TenantID:   tenantID,
		ProjectID:  projectID,
		Name:       req.Name,
		DBType:     req.DBType,
		DSN:        req.DSN,
		ConfigJSON: req.ConfigJSON,
		CreatedBy:  resolveUserID(c, req.CreatedBy),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, datasource)
}

func (h *DatasourceHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	tenantID := parseUint64Default(c.Param("tenant_id"), 0)
	if tenantID == 0 {
		tenantID = parseUint64Default(c.Query("tenant_id"), 0)
	}
	projectID := parseUint64Default(c.Param("project_id"), 0)

	result, err := h.datasourceService.List(c.Request.Context(), tenantID, projectID, page, pageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) TestConnection(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	result, err := h.datasourceService.TestConnection(c.Request.Context(), datasourceID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) TestConnectionWithConfig(c *gin.Context) {
	var req testDatasourceConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.datasourceService.TestConnectionWithConfig(c.Request.Context(), service.TestDatasourceConnectionInput{
		DBType:     req.DBType,
		DSN:        req.DSN,
		ConfigJSON: req.ConfigJSON,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) Delete(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	if err := h.datasourceService.Delete(c.Request.Context(), service.DeleteDatasourceInput{
		TenantID:     tenantID,
		DatasourceID: datasourceID,
	}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}

func (h *DatasourceHandler) SyncMetadata(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	var req syncDatasourceRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.datasourceService.SyncMetadata(c.Request.Context(), service.SyncMetadataInput{
		DatasourceID: datasourceID,
		TriggerType:  req.TriggerType,
		UserID:       resolveUserID(c, req.UserID),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) ListMetadataTables(c *gin.Context) {
	page, pageSize := pageParams(c)
	datasourceID := parseUint64Default(c.Param("id"), 0)
	withColumns := c.Query("with_columns") == "true" || c.Query("with_columns") == "1"

	result, err := h.datasourceService.ListMetadataTables(c.Request.Context(), datasourceID, page, pageSize, withColumns)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) GetMetadataTableDetail(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	tableID := parseUint64Default(c.Param("table_id"), 0)

	result, err := h.datasourceService.GetMetadataTableDetail(c.Request.Context(), datasourceID, tableID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) UpdateMetadataTableComment(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	tableID := parseUint64Default(c.Param("table_id"), 0)
	meta := requestMetadata(c)
	var req updateMetadataCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.datasourceService.UpdateMetadataTableComment(c.Request.Context(), service.UpdateMetadataCommentInput{
		DatasourceID: datasourceID,
		TableID:      tableID,
		Comment:      req.Comment,
		UserID:       resolveUserID(c, req.UserID),
		RequestID:    meta.RequestID,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DatasourceHandler) UpdateMetadataColumnComment(c *gin.Context) {
	datasourceID := parseUint64Default(c.Param("id"), 0)
	columnID := parseUint64Default(c.Param("column_id"), 0)
	meta := requestMetadata(c)
	var req updateMetadataCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.datasourceService.UpdateMetadataColumnComment(c.Request.Context(), service.UpdateMetadataCommentInput{
		DatasourceID: datasourceID,
		ColumnID:     columnID,
		Comment:      req.Comment,
		UserID:       resolveUserID(c, req.UserID),
		RequestID:    meta.RequestID,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
