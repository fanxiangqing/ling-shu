package query

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	blockedSQLPattern   = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|replace|merge|call|exec|grant|revoke)\b`)
	dangerousSQLPattern = regexp.MustCompile(`(?i)\b(sleep|benchmark|load_file|into\s+outfile|into\s+dumpfile)\b`)
	systemSchemaPattern = regexp.MustCompile(`(?i)\b(information_schema|performance_schema|mysql|sys)\s*\.`)
	limitPattern        = regexp.MustCompile(`(?i)\blimit\s+\d+\b`)
	topPattern          = regexp.MustCompile(`(?i)\bselect\s+(distinct\s+)?top\s+\d+\b`)
	fetchPattern        = regexp.MustCompile(`(?i)\bfetch\s+first\s+\d+\s+rows\s+only\b`)
	rownumPattern       = regexp.MustCompile(`(?i)\brownum\s*<=?\s*\d+\b`)
	commentPattern      = regexp.MustCompile(`(?s)/\*.*?\*/|--[^\n\r]*`)
)

type SQLReviewer struct {
	DefaultLimit int
	MaxLimit     int
}

func NewSQLReviewer(defaultLimit int, maxLimit int) *SQLReviewer {
	if defaultLimit <= 0 {
		defaultLimit = 200
	}
	if maxLimit <= 0 {
		maxLimit = 1000
	}
	return &SQLReviewer{DefaultLimit: defaultLimit, MaxLimit: maxLimit}
}

func (r *SQLReviewer) Review(sqlText string) ReviewResult {
	return r.ReviewWithLimit(sqlText, r.DefaultLimit)
}

func (r *SQLReviewer) ReviewWithLimit(sqlText string, limit int) ReviewResult {
	return r.ReviewWithDialect(sqlText, limit, "")
}

func (r *SQLReviewer) ReviewWithDialect(sqlText string, limit int, dialect string) ReviewResult {
	if limit <= 0 {
		limit = r.DefaultLimit
	}
	if limit > r.MaxLimit {
		limit = r.MaxLimit
	}

	normalized := normalizeSQL(sqlText)
	result := ReviewResult{
		Passed:        false,
		RiskLevel:     "low",
		NormalizedSQL: normalized,
		Limit:         limit,
	}

	if normalized == "" {
		result.RiskLevel = "high"
		result.BlockedReason = "SQL 不能为空"
		return result
	}
	if strings.Count(normalized, ";") > 1 || (strings.Contains(normalized, ";") && !strings.HasSuffix(normalized, ";")) {
		result.RiskLevel = "high"
		result.BlockedReason = "禁止多语句 SQL"
		return result
	}

	withoutComments := strings.TrimSpace(commentPattern.ReplaceAllString(normalized, " "))
	lower := strings.ToLower(withoutComments)
	if !strings.HasPrefix(lower, "select ") && !strings.HasPrefix(lower, "with ") {
		result.RiskLevel = "high"
		result.BlockedReason = "仅允许 SELECT 查询"
		return result
	}
	if blockedSQLPattern.MatchString(withoutComments) {
		result.RiskLevel = "high"
		result.BlockedReason = "SQL 包含禁止的写入或结构变更关键字"
		return result
	}
	if dangerousSQLPattern.MatchString(withoutComments) {
		result.RiskLevel = "high"
		result.BlockedReason = "SQL 包含危险函数或文件导出语句"
		return result
	}
	if systemSchemaPattern.MatchString(withoutComments) {
		result.RiskLevel = "high"
		result.BlockedReason = "禁止访问系统库或元数据库"
		return result
	}

	result.Passed = true
	sqlWithoutSuffix := strings.TrimSuffix(normalized, ";")
	result.NormalizedSQL = ensureLimitForDialect(sqlWithoutSuffix, limit, dialect)
	if !hasRowLimit(normalized, dialect) {
		if result.NormalizedSQL == sqlWithoutSuffix {
			result.Warnings = append(result.Warnings, fmt.Sprintf("执行端最多读取 %d 行", limit))
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("已自动追加结果上限 %d", limit))
		}
	}
	return result
}

func normalizeSQL(sqlText string) string {
	fields := strings.Fields(strings.TrimSpace(sqlText))
	return strings.Join(fields, " ")
}

func ensureLimit(sqlText string, limit int) string {
	return ensureLimitForDialect(sqlText, limit, "")
}

func ensureLimitForDialect(sqlText string, limit int, dialect string) string {
	if hasRowLimit(sqlText, dialect) {
		return sqlText
	}
	switch normalizeDialect(dialect) {
	case "oracle":
		return fmt.Sprintf("%s FETCH FIRST %d ROWS ONLY", sqlText, limit)
	case "sqlserver":
		return ensureSQLServerTop(sqlText, limit)
	default:
		return fmt.Sprintf("%s LIMIT %d", sqlText, limit)
	}
}

func hasRowLimit(sqlText string, dialect string) bool {
	switch normalizeDialect(dialect) {
	case "sqlserver":
		return topPattern.MatchString(sqlText) || fetchPattern.MatchString(sqlText)
	case "oracle":
		return fetchPattern.MatchString(sqlText) || rownumPattern.MatchString(sqlText)
	default:
		return limitPattern.MatchString(sqlText)
	}
}

func ensureSQLServerTop(sqlText string, limit int) string {
	lower := strings.ToLower(strings.TrimSpace(sqlText))
	if strings.HasPrefix(lower, "select distinct ") {
		return regexp.MustCompile(`(?i)^select\s+distinct\s+`).ReplaceAllString(sqlText, fmt.Sprintf("SELECT DISTINCT TOP %d ", limit))
	}
	if strings.HasPrefix(lower, "select ") {
		return regexp.MustCompile(`(?i)^select\s+`).ReplaceAllString(sqlText, fmt.Sprintf("SELECT TOP %d ", limit))
	}
	return sqlText
}

func normalizeDialect(dialect string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql", "kingbase", "kingbasees":
		return "postgresql"
	case "sqlserver", "mssql":
		return "sqlserver"
	case "oracle":
		return "oracle"
	case "clickhouse":
		return "clickhouse"
	case "doris":
		return "doris"
	default:
		return "mysql"
	}
}
