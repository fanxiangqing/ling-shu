package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func NewClickHouseDriver() Driver {
	return newSQLQueryDriver("clickhouse", "clickhouse", introspectClickHouse, clickhouseVersion)
}

func clickhouseVersion(ctx context.Context, db *sql.DB) (string, error) {
	return readSingleString(ctx, db, "SELECT version()")
}

func introspectClickHouse(ctx context.Context, db *sql.DB) (*Metadata, error) {
	database, err := clickhouseCurrentDatabase(ctx, db)
	if err != nil {
		return nil, err
	}
	tables, err := clickhouseTables(ctx, db, database)
	if err != nil {
		return nil, err
	}
	columnsByTable, err := clickhouseColumns(ctx, db, database)
	if err != nil {
		return nil, err
	}
	indexesByTable, err := clickhouseIndexes(ctx, db, database)
	if err != nil {
		return nil, err
	}
	tables = attachColumns(tables, columnsByTable)
	tables = attachIndexes(tables, indexesByTable)
	return &Metadata{
		Schemas: []Schema{{Name: database}},
		Tables:  tables,
	}, nil
}

func clickhouseCurrentDatabase(ctx context.Context, db *sql.DB) (string, error) {
	var database string
	if err := db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&database); err != nil {
		return "", fmt.Errorf("read clickhouse current database: %w", err)
	}
	return database, nil
}

func clickhouseTables(ctx context.Context, db *sql.DB, database string) ([]Table, error) {
	rows, err := db.QueryContext(ctx, `
SELECT database,
       name,
       engine,
       comment,
       total_rows
FROM system.tables
WHERE database = ?
  AND is_temporary = 0
ORDER BY database, name`, database)
	if err != nil {
		return nil, fmt.Errorf("query clickhouse tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var schema, name, engine string
		var comment sql.NullString
		var rowCount sql.NullInt64
		if err := rows.Scan(&schema, &name, &engine, &comment, &rowCount); err != nil {
			return nil, fmt.Errorf("scan clickhouse table: %w", err)
		}
		tableType := "table"
		if strings.Contains(strings.ToLower(engine), "view") {
			tableType = "view"
		}
		table := Table{
			Schema:  schema,
			Name:    name,
			Type:    tableType,
			Comment: comment.String,
		}
		if rowCount.Valid {
			value := rowCount.Int64
			table.RowCount = &value
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clickhouse tables: %w", err)
	}
	return tables, nil
}

func clickhouseColumns(ctx context.Context, db *sql.DB, database string) (map[string][]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT database,
       table,
       name,
       position,
       type,
       type,
       default_expression,
       comment
FROM system.columns
WHERE database = ?
ORDER BY database, table, position`, database)
	if err != nil {
		return nil, fmt.Errorf("query clickhouse columns: %w", err)
	}
	defer rows.Close()

	columnsByTable := map[string][]Column{}
	for rows.Next() {
		var schema, tableName, columnName, dataType, columnType string
		var ordinal int
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&schema, &tableName, &columnName, &ordinal, &dataType, &columnType, &defaultValue, &comment); err != nil {
			return nil, fmt.Errorf("scan clickhouse column: %w", err)
		}
		key := tableKey(schema, tableName)
		columnsByTable[key] = append(columnsByTable[key], Column{
			Name:            columnName,
			OrdinalPosition: ordinal,
			DataType:        dataType,
			ColumnType:      columnType,
			Nullable:        strings.HasPrefix(strings.ToLower(dataType), "nullable("),
			DefaultValue:    defaultValue.String,
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clickhouse columns: %w", err)
	}
	return columnsByTable, nil
}

func clickhouseIndexes(ctx context.Context, db *sql.DB, database string) (map[string][]Index, error) {
	rows, err := db.QueryContext(ctx, `
SELECT database,
       name,
       primary_key,
       sorting_key
FROM system.tables
WHERE database = ?
  AND is_temporary = 0
ORDER BY database, name`, database)
	if err != nil {
		return nil, fmt.Errorf("query clickhouse keys: %w", err)
	}
	defer rows.Close()

	out := map[string][]Index{}
	for rows.Next() {
		var schema, tableName string
		var primaryKey, sortingKey sql.NullString
		if err := rows.Scan(&schema, &tableName, &primaryKey, &sortingKey); err != nil {
			return nil, fmt.Errorf("scan clickhouse key: %w", err)
		}
		key := tableKey(schema, tableName)
		if strings.TrimSpace(primaryKey.String) != "" {
			out[key] = append(out[key], Index{
				Name:    "_primary_key",
				Type:    "primary_key",
				Columns: splitClickHouseKeyExpression(primaryKey.String),
			})
		}
		if strings.TrimSpace(sortingKey.String) != "" && sortingKey.String != primaryKey.String {
			out[key] = append(out[key], Index{
				Name:    "_sorting_key",
				Type:    "sorting_key",
				Columns: splitClickHouseKeyExpression(sortingKey.String),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clickhouse keys: %w", err)
	}
	return out, nil
}

func splitClickHouseKeyExpression(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimSuffix(value, ")"), "(")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
