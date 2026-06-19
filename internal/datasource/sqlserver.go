package datasource

import (
	"context"
	"database/sql"
	"fmt"
)

func NewSQLServerDriver() Driver {
	return newSQLQueryDriver("sqlserver", "sqlserver", introspectSQLServer, sqlserverVersion)
}

func sqlserverVersion(ctx context.Context, db *sql.DB) (string, error) {
	return readSingleString(ctx, db, "SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(128))")
}

func introspectSQLServer(ctx context.Context, db *sql.DB) (*Metadata, error) {
	tables, err := sqlserverTables(ctx, db)
	if err != nil {
		return nil, err
	}
	columnsByTable, err := sqlserverColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	indexesByTable, err := sqlserverIndexes(ctx, db)
	if err != nil {
		return nil, err
	}
	foreignKeysByTable, err := sqlserverForeignKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	tables = attachColumns(tables, columnsByTable)
	tables = attachIndexes(tables, indexesByTable)
	tables = attachForeignKeys(tables, foreignKeysByTable)
	return buildMetadata(tables), nil
}

func sqlserverTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	rows, err := db.QueryContext(ctx, `
SELECT t.TABLE_SCHEMA,
       t.TABLE_NAME,
       t.TABLE_TYPE,
       COALESCE(CAST(ep.value AS nvarchar(4000)), '') AS TABLE_COMMENT,
       CONVERT(bigint, NULL) AS TABLE_ROWS
FROM INFORMATION_SCHEMA.TABLES t
LEFT JOIN sys.schemas s ON s.name = t.TABLE_SCHEMA
LEFT JOIN sys.objects o ON o.name = t.TABLE_NAME AND o.schema_id = s.schema_id
LEFT JOIN sys.extended_properties ep
  ON ep.major_id = o.object_id AND ep.minor_id = 0 AND ep.name = 'MS_Description'
WHERE t.TABLE_TYPE IN ('BASE TABLE', 'VIEW')
ORDER BY t.TABLE_SCHEMA, t.TABLE_NAME`)
	if err != nil {
		return nil, fmt.Errorf("query sqlserver tables: %w", err)
	}
	defer rows.Close()
	return scanTableRows(rows, "sqlserver")
}

func sqlserverColumns(ctx context.Context, db *sql.DB) (map[string][]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT c.TABLE_SCHEMA,
       c.TABLE_NAME,
       c.COLUMN_NAME,
       c.ORDINAL_POSITION,
       c.DATA_TYPE,
       COALESCE(c.DATA_TYPE +
         CASE
           WHEN c.CHARACTER_MAXIMUM_LENGTH IS NULL THEN ''
           WHEN c.CHARACTER_MAXIMUM_LENGTH = -1 THEN '(max)'
           ELSE '(' + CAST(c.CHARACTER_MAXIMUM_LENGTH AS varchar(20)) + ')'
         END, c.DATA_TYPE) AS COLUMN_TYPE,
       c.IS_NULLABLE,
       COALESCE(c.COLUMN_DEFAULT, '') AS COLUMN_DEFAULT,
       CASE WHEN pk.COLUMN_NAME IS NULL THEN '' ELSE 'PRI' END AS COLUMN_KEY,
       CASE WHEN fk.COLUMN_NAME IS NULL THEN '' ELSE 'FK' END AS FOREIGN_KEY,
       COALESCE(CAST(ep.value AS nvarchar(4000)), '') AS COLUMN_COMMENT
FROM INFORMATION_SCHEMA.COLUMNS c
LEFT JOIN (
  SELECT ku.TABLE_SCHEMA, ku.TABLE_NAME, ku.COLUMN_NAME
  FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
  JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
    ON ku.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
   AND ku.TABLE_SCHEMA = tc.TABLE_SCHEMA
   AND ku.TABLE_NAME = tc.TABLE_NAME
  WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
) pk ON pk.TABLE_SCHEMA = c.TABLE_SCHEMA
    AND pk.TABLE_NAME = c.TABLE_NAME
    AND pk.COLUMN_NAME = c.COLUMN_NAME
LEFT JOIN (
  SELECT ku.TABLE_SCHEMA, ku.TABLE_NAME, ku.COLUMN_NAME
  FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
  JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
    ON ku.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
   AND ku.TABLE_SCHEMA = tc.TABLE_SCHEMA
   AND ku.TABLE_NAME = tc.TABLE_NAME
  WHERE tc.CONSTRAINT_TYPE = 'FOREIGN KEY'
) fk ON fk.TABLE_SCHEMA = c.TABLE_SCHEMA
    AND fk.TABLE_NAME = c.TABLE_NAME
    AND fk.COLUMN_NAME = c.COLUMN_NAME
LEFT JOIN sys.schemas s ON s.name = c.TABLE_SCHEMA
LEFT JOIN sys.objects o ON o.name = c.TABLE_NAME AND o.schema_id = s.schema_id
LEFT JOIN sys.columns sc ON sc.object_id = o.object_id AND sc.name = c.COLUMN_NAME
LEFT JOIN sys.extended_properties ep
  ON ep.major_id = o.object_id AND ep.minor_id = sc.column_id AND ep.name = 'MS_Description'
ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`)
	if err != nil {
		return nil, fmt.Errorf("query sqlserver columns: %w", err)
	}
	defer rows.Close()

	columnsByTable := map[string][]Column{}
	for rows.Next() {
		var tableSchema, tableName, columnName, dataType, columnType, nullable, columnKey, foreignKey string
		var ordinal int
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &columnName, &ordinal, &dataType, &columnType, &nullable, &defaultValue, &columnKey, &foreignKey, &comment); err != nil {
			return nil, fmt.Errorf("scan sqlserver column: %w", err)
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
			IsForeignKey:    foreignKey == "FK",
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlserver columns: %w", err)
	}
	return columnsByTable, nil
}

func sqlserverIndexes(ctx context.Context, db *sql.DB) (map[string][]Index, error) {
	rows, err := db.QueryContext(ctx, `
SELECT s.name AS table_schema,
       o.name AS table_name,
       i.name AS index_name,
       i.type_desc AS index_type,
       i.is_unique AS unique_index,
       c.name AS column_name
FROM sys.indexes i
JOIN sys.objects o ON o.object_id = i.object_id
JOIN sys.schemas s ON s.schema_id = o.schema_id
LEFT JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
LEFT JOIN sys.columns c ON c.object_id = i.object_id AND c.column_id = ic.column_id
WHERE o.type IN ('U', 'V')
  AND i.name IS NOT NULL
ORDER BY s.name, o.name, i.name, ic.key_ordinal`)
	if err != nil {
		return nil, fmt.Errorf("query sqlserver indexes: %w", err)
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
		var schema, tableName, indexName, indexType string
		var unique bool
		var columnName sql.NullString
		if err := rows.Scan(&schema, &tableName, &indexName, &indexType, &unique, &columnName); err != nil {
			return nil, fmt.Errorf("scan sqlserver index: %w", err)
		}
		key := indexKey{schema: schema, table: tableName, name: indexName}
		index, ok := grouped[key]
		if !ok {
			index = &Index{Name: indexName, Type: indexType, Unique: unique}
			grouped[key] = index
			order = append(order, key)
		}
		if columnName.Valid && columnName.String != "" {
			index.Columns = append(index.Columns, columnName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlserver indexes: %w", err)
	}
	out := map[string][]Index{}
	for _, key := range order {
		out[tableKey(key.schema, key.table)] = append(out[tableKey(key.schema, key.table)], *grouped[key])
	}
	return out, nil
}

func sqlserverForeignKeys(ctx context.Context, db *sql.DB) (map[string][]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT s.name AS table_schema,
       t.name AS table_name,
       fk.name AS constraint_name,
       c.name AS column_name,
       rs.name AS referenced_schema,
       rt.name AS referenced_table,
       rc.name AS referenced_column
FROM sys.foreign_keys fk
JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
JOIN sys.tables t ON t.object_id = fk.parent_object_id
JOIN sys.schemas s ON s.schema_id = t.schema_id
JOIN sys.columns c ON c.object_id = t.object_id AND c.column_id = fkc.parent_column_id
JOIN sys.tables rt ON rt.object_id = fk.referenced_object_id
JOIN sys.schemas rs ON rs.schema_id = rt.schema_id
JOIN sys.columns rc ON rc.object_id = rt.object_id AND rc.column_id = fkc.referenced_column_id
ORDER BY s.name, t.name, fk.name, fkc.constraint_column_id`)
	if err != nil {
		return nil, fmt.Errorf("query sqlserver foreign keys: %w", err)
	}
	defer rows.Close()

	out := map[string][]ForeignKey{}
	for rows.Next() {
		var schema, tableName string
		var fk ForeignKey
		if err := rows.Scan(&schema, &tableName, &fk.ConstraintName, &fk.ColumnName, &fk.ReferencedSchema, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("scan sqlserver foreign key: %w", err)
		}
		out[tableKey(schema, tableName)] = append(out[tableKey(schema, tableName)], fk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlserver foreign keys: %w", err)
	}
	return out, nil
}
