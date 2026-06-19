package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ling-shu/internal/datasource"
	"ling-shu/internal/middleware"
	"ling-shu/internal/repository"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func pageParams(c *gin.Context) (int, int) {
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	return page, pageSize
}

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseUint64Default(value string, fallback uint64) uint64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseOptionalBool(value string) *bool {
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseOptionalTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed
		}
	}
	if millis, err := strconv.ParseInt(value, 10, 64); err == nil && millis > 0 {
		return time.UnixMilli(millis)
	}
	return time.Time{}
}

type requestMeta struct {
	RequestID string
	IP        string
	UserAgent string
}

func requestMetadata(c *gin.Context) requestMeta {
	requestIDValue, _ := c.Get(middleware.RequestIDKey)
	requestID, _ := requestIDValue.(string)
	return requestMeta{
		RequestID: requestID,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
}

func authenticatedUserID(c *gin.Context) uint64 {
	value, ok := c.Get(middleware.UserIDKey)
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case uint64:
		return typed
	case uint:
		return uint64(typed)
	case int:
		if typed > 0 {
			return uint64(typed)
		}
	}
	return 0
}

func resolveUserID(c *gin.Context, fallback uint64) uint64 {
	if userID := authenticatedUserID(c); userID > 0 {
		return userID
	}
	return fallback
}

func writeError(c *gin.Context, err error) {
	_ = c.Error(err)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid username or password")
	case errors.Is(err, service.ErrUserDisabled):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "user is disabled")
	case errors.Is(err, service.ErrNoActiveWorkspace):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "user has no active workspace")
	case errors.Is(err, service.ErrPrimaryAdminLocked):
		response.Error(c, http.StatusConflict, response.CodeConflict, "primary admin cannot be modified")
	case errors.Is(err, service.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid input")
	case errors.Is(err, service.ErrDatasourceInUse):
		response.Error(c, http.StatusConflict, response.CodeConflict, "数据源已被项目引用，请先调整或删除相关项目后再删除数据源")
	case errors.Is(err, service.ErrQueryAlreadyRunning):
		response.Error(c, http.StatusConflict, response.CodeConflict, "相同查询正在执行，请稍后查看结果或重新提问")
	case errors.Is(err, service.ErrSecretEncryptFailed):
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "数据源连接信息加密失败，请检查服务端密钥配置")
	case errors.Is(err, service.ErrSecretDecryptFailed):
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "数据源连接信息解密失败，请检查 LING_SHU_DSN_SECRET 是否和创建数据源时一致")
	case errors.Is(err, datasource.ErrDriverNotFound), errors.Is(err, datasource.ErrInvalidConfig):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, repository.ErrDatabaseDisabled):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is disabled")
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "record not found")
	case isDuplicateEntryError(err):
		response.Error(c, http.StatusConflict, response.CodeConflict, "记录已存在，请检查名称、编码或账号后再试")
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
	}
}

func isDuplicateEntryError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate entry") || strings.Contains(message, "error 1062")
}
