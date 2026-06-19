package response

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

const (
	CodeOK                 = 0
	CodeBadRequest         = 40000
	CodeUnauthorized       = 40100
	CodeForbidden          = 40300
	CodeNotFound           = 40400
	CodeConflict           = 40900
	CodeTooManyRequests    = 42900
	CodeServiceUnavailable = 50300
	CodeInternal           = 50000
)

type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeOK, "ok", data)
}

func Error(c *gin.Context, status int, code int, message string) {
	JSON(c, status, code, FriendlyMessage(message), nil)
}

func JSON(c *gin.Context, status int, code int, message string, data any) {
	requestID, _ := c.Get("request_id")
	body := Body{
		Code:    code,
		Message: message,
		Data:    data,
	}
	if requestIDString, ok := requestID.(string); ok {
		body.RequestID = requestIDString
	}
	c.JSON(status, body)
}

func FriendlyMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "请求处理失败，请稍后重试"
	}
	if containsCJK(message) {
		return message
	}
	lower := strings.ToLower(message)
	exact := map[string]string{
		"ok":                                        "ok",
		"invalid input":                             "请求参数不完整或不合法，请检查后再试",
		"invalid request body":                      "请求内容格式不正确，请检查后再试",
		"invalid request query":                     "请求参数不完整或格式不正确，请检查后再试",
		"request failed":                            "请求失败，请稍后重试",
		"stream failed":                             "流式请求失败，请稍后重试",
		"stream response is not readable":           "服务没有返回可读取的流式响应，请稍后重试",
		"stream finished without result":            "流式请求已结束，但没有返回最终结果，请稍后重试",
		"service call failed":                       "服务调用失败，请稍后重试",
		"model service call failed":                 "模型服务调用失败，请稍后重试",
		"service is not configured":                 "服务尚未配置，请先完成相关配置",
		"provider is not configured":                "服务尚未配置，请先完成相关配置",
		"provider streaming audio is not supported": "当前语音服务不支持流式音频",
		"llm provider is not configured":            "大模型服务未配置，请先配置 LLM",
		"model service is not configured":           "大模型服务未配置，请先配置 LLM",
		"prompt renderer is not configured":         "Prompt 模板服务未配置，请检查服务端配置",
		"rag provider is not configured":            "知识库服务未配置，请先配置 RAG",
		"database is disabled":                      "元数据库未启用，请检查服务端配置",
		"record not found":                          "记录不存在或已被删除",
		"internal server error":                     "服务暂时异常，请稍后重试",
		"auth is not configured":                    "认证服务未配置，请联系管理员",
		"missing bearer token":                      "请先登录后再操作",
		"invalid bearer token":                      "登录状态已失效，请重新登录",
		"invalid authorization header":              "登录凭证格式不正确，请重新登录",
		"authentication is required":                "请先登录后再操作",
		"permission checker is not configured":      "权限服务未配置，请联系管理员",
		"permission denied":                         "没有权限执行该操作",
		"invalid permission scope":                  "权限范围参数不正确",
		"tenant_id is required":                     "请选择组织后再操作",
		"project_id is required":                    "请选择项目后再操作",
		"datasource id is required":                 "请选择数据源后再操作",
		"datasource scope not found":                "数据源不存在或没有访问权限",
		"check permission failed":                   "权限校验失败，请稍后重试",
		"dsn is required":                           "数据源连接信息不能为空",
	}
	if value, ok := exact[lower]; ok {
		return value
	}
	switch {
	case strings.Contains(lower, "datasource driver not found"):
		return "暂不支持该数据源类型，请检查数据库类型是否正确"
	case strings.Contains(lower, "invalid datasource config"):
		return "数据源配置不正确，请检查连接信息"
	case strings.Contains(lower, "duplicate entry") || strings.Contains(lower, "error 1062"):
		return "记录已存在，请检查名称、编码或账号后再试"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "context deadline"):
		return "请求处理时间较长，本次已中断。可以缩小问题范围、减少结果数量，或稍后重试"
	case strings.Contains(lower, "too many requests") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "throttl"):
		return "服务当前繁忙，请稍后重试"
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "invalid api key") || strings.Contains(lower, "authentication"):
		return "服务认证失败，请检查配置或重新登录"
	case strings.Contains(lower, "network error"):
		return "无法连接服务，请检查后端是否启动"
	case strings.Contains(lower, "failed to fetch"):
		return "请求服务失败，请检查网络或后端服务状态"
	default:
		return message
	}
}

func containsCJK(message string) bool {
	for _, r := range message {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
