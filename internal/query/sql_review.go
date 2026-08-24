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

	commentFreeSQL := stripSQLComments(sqlText)
	normalized := normalizeSQL(commentFreeSQL)
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
	if hasMultipleSQLStatements(commentFreeSQL) {
		result.RiskLevel = "high"
		result.BlockedReason = "禁止多语句 SQL"
		return result
	}

	lower := strings.ToLower(normalized)
	if !strings.HasPrefix(lower, "select ") && !strings.HasPrefix(lower, "with ") {
		result.RiskLevel = "high"
		result.BlockedReason = "仅允许 SELECT 查询"
		return result
	}
	if blockedSQLPattern.MatchString(normalized) {
		result.RiskLevel = "high"
		result.BlockedReason = "SQL 包含禁止的写入或结构变更关键字"
		return result
	}
	if dangerousSQLPattern.MatchString(normalized) {
		result.RiskLevel = "high"
		result.BlockedReason = "SQL 包含危险函数或文件导出语句"
		return result
	}
	if systemSchemaPattern.MatchString(normalized) {
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

func stripSQLComments(sqlText string) string {
	var out strings.Builder
	out.Grow(len(sqlText))
	for index := 0; index < len(sqlText); index++ {
		switch sqlText[index] {
		case '\'':
			end := scanQuotedSQL(sqlText, index, '\'', true)
			out.WriteString(sqlText[index : end+1])
			index = end
		case '"':
			end := scanQuotedSQL(sqlText, index, '"', true)
			out.WriteString(sqlText[index : end+1])
			index = end
		case '`':
			end := scanQuotedSQL(sqlText, index, '`', false)
			out.WriteString(sqlText[index : end+1])
			index = end
		case '[':
			end := scanBracketQuotedSQL(sqlText, index)
			out.WriteString(sqlText[index : end+1])
			index = end
		case '-':
			if index+1 < len(sqlText) && sqlText[index+1] == '-' {
				out.WriteByte(' ')
				index = scanLineCommentSQL(sqlText, index+2)
				continue
			}
			out.WriteByte(sqlText[index])
		case '/':
			if index+1 < len(sqlText) && sqlText[index+1] == '*' {
				out.WriteByte(' ')
				index = scanBlockCommentSQL(sqlText, index+2)
				continue
			}
			out.WriteByte(sqlText[index])
		default:
			out.WriteByte(sqlText[index])
		}
	}
	return out.String()
}

func hasMultipleSQLStatements(sqlText string) bool {
	for index := 0; index < len(sqlText); index++ {
		switch sqlText[index] {
		case '\'':
			index = scanQuotedSQL(sqlText, index, '\'', true)
		case '"':
			index = scanQuotedSQL(sqlText, index, '"', true)
		case '`':
			index = scanQuotedSQL(sqlText, index, '`', false)
		case '[':
			index = scanBracketQuotedSQL(sqlText, index)
		case '-':
			if index+1 < len(sqlText) && sqlText[index+1] == '-' {
				index = scanLineCommentSQL(sqlText, index+2)
			}
		case '/':
			if index+1 < len(sqlText) && sqlText[index+1] == '*' {
				index = scanBlockCommentSQL(sqlText, index+2)
			}
		case ';':
			return hasSQLCodeAfterTerminator(sqlText[index+1:])
		}
	}
	return false
}

func hasSQLCodeAfterTerminator(sqlText string) bool {
	for index := 0; index < len(sqlText); index++ {
		switch sqlText[index] {
		case ' ', '\t', '\n', '\r', '\f':
			continue
		case '-':
			if index+1 < len(sqlText) && sqlText[index+1] == '-' {
				index = scanLineCommentSQL(sqlText, index+2)
				continue
			}
			return true
		case '/':
			if index+1 < len(sqlText) && sqlText[index+1] == '*' {
				index = scanBlockCommentSQL(sqlText, index+2)
				continue
			}
			return true
		default:
			return true
		}
	}
	return false
}

func scanQuotedSQL(sqlText string, start int, quote byte, allowBackslashEscape bool) int {
	for index := start + 1; index < len(sqlText); index++ {
		if allowBackslashEscape && sqlText[index] == '\\' && index+1 < len(sqlText) {
			index++
			continue
		}
		if sqlText[index] != quote {
			continue
		}
		if index+1 < len(sqlText) && sqlText[index+1] == quote {
			index++
			continue
		}
		return index
	}
	return len(sqlText) - 1
}

func scanBracketQuotedSQL(sqlText string, start int) int {
	for index := start + 1; index < len(sqlText); index++ {
		if sqlText[index] == ']' {
			return index
		}
	}
	return len(sqlText) - 1
}

func scanLineCommentSQL(sqlText string, start int) int {
	for index := start; index < len(sqlText); index++ {
		if sqlText[index] == '\n' || sqlText[index] == '\r' {
			return index
		}
	}
	return len(sqlText) - 1
}

func scanBlockCommentSQL(sqlText string, start int) int {
	for index := start; index+1 < len(sqlText); index++ {
		if sqlText[index] == '*' && sqlText[index+1] == '/' {
			return index + 1
		}
	}
	return len(sqlText) - 1
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
