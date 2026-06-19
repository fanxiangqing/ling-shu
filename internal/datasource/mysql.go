package datasource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLDriver struct{}

func NewMySQLDriver() *MySQLDriver {
	return &MySQLDriver{}
}

func (d *MySQLDriver) Type() string {
	return "mysql"
}

func (d *MySQLDriver) Ping(ctx context.Context, cfg Config) error {
	db, err := openMySQL(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (d *MySQLDriver) Version(ctx context.Context, cfg Config) (string, error) {
	db, err := openMySQL(cfg)
	if err != nil {
		return "", err
	}
	defer db.Close()
	return readSingleString(ctx, db, "SELECT VERSION()")
}

func (d *MySQLDriver) Introspect(ctx context.Context, cfg Config) (*Metadata, error) {
	db, err := openMySQL(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	schema, err := currentDatabase(ctx, db)
	if err != nil {
		return nil, err
	}
	if schema == "" {
		return nil, errors.New("mysql database name is required in dsn")
	}

	tables, err := mysqlTables(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	columnsByTable, err := mysqlColumns(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	indexesByTable, err := mysqlIndexes(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	foreignKeysByTable, err := mysqlForeignKeys(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		key := tableKey(tables[i].Schema, tables[i].Name)
		tables[i].Columns = columnsByTable[key]
		tables[i].Indexes = indexesByTable[key]
		tables[i].ForeignKeys = foreignKeysByTable[key]
	}

	metadata := &Metadata{
		Schemas: []Schema{{Name: schema}},
		Tables:  tables,
	}
	if version, versionErr := readSingleString(ctx, db, "SELECT VERSION()"); versionErr == nil {
		metadata.Version = version
	}
	return metadata, nil
}

func (d *MySQLDriver) Query(ctx context.Context, cfg Config, sqlText string, maxRows int) (*QueryResult, error) {
	if maxRows <= 0 {
		maxRows = 200
	}
	db, err := openMySQL(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("query mysql datasource: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read mysql result columns: %w", err)
	}

	result := &QueryResult{
		Columns: columns,
		Rows:    make([]map[string]any, 0, maxRows),
	}
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			break
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("scan mysql result row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeSQLValue(values[i])
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql result rows: %w", err)
	}
	return result, nil
}

func openMySQL(cfg Config) (*sql.DB, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, errors.New("dsn is required")
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql datasource: %w", err)
	}
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	return db, nil
}

func currentDatabase(ctx context.Context, db *sql.DB) (string, error) {
	var schema sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
		return "", fmt.Errorf("read current database: %w", err)
	}
	return schema.String, nil
}

func mysqlTables(ctx context.Context, db *sql.DB, schema string) ([]Table, error) {
	rows, err := db.QueryContext(ctx, `
SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, TABLE_COMMENT, TABLE_ROWS
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ?
  AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
ORDER BY TABLE_NAME`, schema)
	if err != nil {
		return nil, fmt.Errorf("query mysql tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var tableSchema, tableName, tableType string
		var comment sql.NullString
		var rowCount sql.NullInt64
		if err := rows.Scan(&tableSchema, &tableName, &tableType, &comment, &rowCount); err != nil {
			return nil, fmt.Errorf("scan mysql table: %w", err)
		}
		table := Table{
			Schema:  tableSchema,
			Name:    tableName,
			Type:    normalizeTableType(tableType),
			Comment: comment.String,
		}
		if rowCount.Valid {
			value := rowCount.Int64
			table.RowCount = &value
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql tables: %w", err)
	}
	return tables, nil
}

func mysqlColumns(ctx context.Context, db *sql.DB, schema string) (map[string][]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, ORDINAL_POSITION, DATA_TYPE, COLUMN_TYPE,
       IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY, COLUMN_COMMENT
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, ORDINAL_POSITION`, schema)
	if err != nil {
		return nil, fmt.Errorf("query mysql columns: %w", err)
	}
	defer rows.Close()

	columnsByTable := map[string][]Column{}
	for rows.Next() {
		var tableSchema, tableName, columnName, dataType, columnType, nullable, columnKey string
		var ordinal int
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &columnName, &ordinal, &dataType, &columnType, &nullable, &defaultValue, &columnKey, &comment); err != nil {
			return nil, fmt.Errorf("scan mysql column: %w", err)
		}
		key := tableKey(tableSchema, tableName)
		columnsByTable[key] = append(columnsByTable[key], Column{
			Name:            columnName,
			OrdinalPosition: ordinal,
			DataType:        dataType,
			ColumnType:      columnType,
			Nullable:        nullable == "YES",
			DefaultValue:    defaultValue.String,
			IsPrimaryKey:    columnKey == "PRI",
			IsForeignKey:    columnKey == "MUL",
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql columns: %w", err)
	}
	return columnsByTable, nil
}

func mysqlIndexes(ctx context.Context, db *sql.DB, schema string) (map[string][]Index, error) {
	rows, err := db.QueryContext(ctx, `
SELECT TABLE_SCHEMA, TABLE_NAME, INDEX_NAME, INDEX_TYPE, NON_UNIQUE, COLUMN_NAME
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, schema)
	if err != nil {
		return nil, fmt.Errorf("query mysql indexes: %w", err)
	}
	defer rows.Close()

	type indexKey struct {
		schema string
		table  string
		name   string
	}
	grouped := map[indexKey]*Index{}
	order := make([]indexKey, 0)
	for rows.Next() {
		var tableSchema, tableName, indexName string
		var indexType, columnName sql.NullString
		var nonUnique sql.NullInt64
		if err := rows.Scan(&tableSchema, &tableName, &indexName, &indexType, &nonUnique, &columnName); err != nil {
			return nil, fmt.Errorf("scan mysql index: %w", err)
		}
		key := indexKey{schema: tableSchema, table: tableName, name: indexName}
		index, ok := grouped[key]
		if !ok {
			index = &Index{
				Name:   indexName,
				Type:   indexType.String,
				Unique: nonUnique.Valid && nonUnique.Int64 == 0,
			}
			grouped[key] = index
			order = append(order, key)
		}
		if columnName.Valid && strings.TrimSpace(columnName.String) != "" {
			index.Columns = append(index.Columns, columnName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql indexes: %w", err)
	}
	out := map[string][]Index{}
	for _, key := range order {
		out[tableKey(key.schema, key.table)] = append(out[tableKey(key.schema, key.table)], *grouped[key])
	}
	return out, nil
}

func mysqlForeignKeys(ctx context.Context, db *sql.DB, schema string) (map[string][]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT TABLE_SCHEMA, TABLE_NAME, CONSTRAINT_NAME, COLUMN_NAME,
       REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = ?
  AND REFERENCED_TABLE_NAME IS NOT NULL
ORDER BY TABLE_NAME, CONSTRAINT_NAME, ORDINAL_POSITION`, schema)
	if err != nil {
		return nil, fmt.Errorf("query mysql foreign keys: %w", err)
	}
	defer rows.Close()

	out := map[string][]ForeignKey{}
	for rows.Next() {
		var tableSchema, tableName, constraintName, columnName string
		var referencedSchema, referencedTable, referencedColumn sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &constraintName, &columnName, &referencedSchema, &referencedTable, &referencedColumn); err != nil {
			return nil, fmt.Errorf("scan mysql foreign key: %w", err)
		}
		key := tableKey(tableSchema, tableName)
		out[key] = append(out[key], ForeignKey{
			ConstraintName:   constraintName,
			ColumnName:       columnName,
			ReferencedSchema: referencedSchema.String,
			ReferencedTable:  referencedTable.String,
			ReferencedColumn: referencedColumn.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql foreign keys: %w", err)
	}
	return out, nil
}

func tableKey(schema string, table string) string {
	return schema + "." + table
}

func normalizeTableType(tableType string) string {
	switch strings.ToUpper(tableType) {
	case "VIEW":
		return "view"
	default:
		return "table"
	}
}

func normalizeSQLValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return typed
	}
}
