package datasource

import (
	"context"
	"database/sql"
	"fmt"
)

func NewOracleDriver() Driver {
	return newSQLQueryDriver("oracle", "oracle", introspectOracle, oracleVersion)
}

func oracleVersion(ctx context.Context, db *sql.DB) (string, error) {
	return readSingleString(ctx, db, "SELECT banner FROM v$version WHERE banner LIKE 'Oracle Database%' AND ROWNUM = 1")
}

func introspectOracle(ctx context.Context, db *sql.DB) (*Metadata, error) {
	schema, err := oracleCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	tables, err := oracleTables(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	columnsByTable, err := oracleColumns(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	indexesByTable, err := oracleIndexes(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	foreignKeysByTable, err := oracleForeignKeys(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	tables = attachColumns(tables, columnsByTable)
	tables = attachIndexes(tables, indexesByTable)
	tables = attachForeignKeys(tables, foreignKeysByTable)
	return buildMetadata(tables), nil
}

func oracleCurrentSchema(ctx context.Context, db *sql.DB) (string, error) {
	var schema string
	if err := db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM dual").Scan(&schema); err != nil {
		return "", fmt.Errorf("read oracle current schema: %w", err)
	}
	return schema, nil
}

func oracleTables(ctx context.Context, db *sql.DB, schema string) ([]Table, error) {
	rows, err := db.QueryContext(ctx, `
SELECT table_schema, table_name, table_type, table_comment, table_rows
FROM (
  SELECT t.owner AS table_schema,
         t.table_name,
         'BASE TABLE' AS table_type,
         NVL(c.comments, '') AS table_comment,
         t.num_rows AS table_rows
  FROM all_tables t
  LEFT JOIN all_tab_comments c ON c.owner = t.owner AND c.table_name = t.table_name
  WHERE t.owner = :1
  UNION ALL
  SELECT v.owner AS table_schema,
         v.view_name AS table_name,
         'VIEW' AS table_type,
         NVL(c.comments, '') AS table_comment,
         CAST(NULL AS NUMBER) AS table_rows
  FROM all_views v
  LEFT JOIN all_tab_comments c ON c.owner = v.owner AND c.table_name = v.view_name
  WHERE v.owner = :1
)
ORDER BY table_schema, table_name`, schema)
	if err != nil {
		return nil, fmt.Errorf("query oracle tables: %w", err)
	}
	defer rows.Close()
	return scanTableRows(rows, "oracle")
}

func oracleColumns(ctx context.Context, db *sql.DB, schema string) (map[string][]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT c.owner,
       c.table_name,
       c.column_name,
       c.column_id,
       c.data_type,
       c.data_type ||
         CASE
           WHEN c.data_type IN ('VARCHAR2', 'NVARCHAR2', 'CHAR', 'NCHAR') THEN '(' || c.char_length || ')'
           WHEN c.data_precision IS NOT NULL AND c.data_scale IS NOT NULL THEN '(' || c.data_precision || ',' || c.data_scale || ')'
           WHEN c.data_precision IS NOT NULL THEN '(' || c.data_precision || ')'
           ELSE ''
         END AS column_type,
       c.nullable,
       '' AS column_default,
       CASE WHEN pk.column_name IS NULL THEN '' ELSE 'PRI' END AS column_key,
       CASE WHEN fk.column_name IS NULL THEN '' ELSE 'FK' END AS foreign_key,
       NVL(cc.comments, '') AS column_comment
FROM all_tab_columns c
LEFT JOIN (
  SELECT acc.owner, acc.table_name, acc.column_name
  FROM all_constraints ac
  JOIN all_cons_columns acc
    ON acc.owner = ac.owner
   AND acc.constraint_name = ac.constraint_name
   AND acc.table_name = ac.table_name
  WHERE ac.constraint_type = 'P'
) pk ON pk.owner = c.owner AND pk.table_name = c.table_name AND pk.column_name = c.column_name
LEFT JOIN (
  SELECT acc.owner, acc.table_name, acc.column_name
  FROM all_constraints ac
  JOIN all_cons_columns acc
    ON acc.owner = ac.owner
   AND acc.constraint_name = ac.constraint_name
   AND acc.table_name = ac.table_name
  WHERE ac.constraint_type = 'R'
) fk ON fk.owner = c.owner AND fk.table_name = c.table_name AND fk.column_name = c.column_name
LEFT JOIN all_col_comments cc
  ON cc.owner = c.owner AND cc.table_name = c.table_name AND cc.column_name = c.column_name
WHERE c.owner = :1
ORDER BY c.owner, c.table_name, c.column_id`, schema)
	if err != nil {
		return nil, fmt.Errorf("query oracle columns: %w", err)
	}
	defer rows.Close()

	columnsByTable := map[string][]Column{}
	for rows.Next() {
		var tableSchema, tableName, columnName, dataType, columnType, nullable string
		var ordinal int
		var defaultValue, columnKey, foreignKey, comment sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &columnName, &ordinal, &dataType, &columnType, &nullable, &defaultValue, &columnKey, &foreignKey, &comment); err != nil {
			return nil, fmt.Errorf("scan oracle column: %w", err)
		}
		key := tableKey(tableSchema, tableName)
		columnsByTable[key] = append(columnsByTable[key], Column{
			Name:            columnName,
			OrdinalPosition: ordinal,
			DataType:        dataType,
			ColumnType:      columnType,
			Nullable:        nullable == "Y",
			DefaultValue:    defaultValue.String,
			IsPrimaryKey:    columnKey.String == "PRI",
			IsForeignKey:    foreignKey.String == "FK",
			Comment:         comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oracle columns: %w", err)
	}
	return columnsByTable, nil
}

func oracleIndexes(ctx context.Context, db *sql.DB, schema string) (map[string][]Index, error) {
	rows, err := db.QueryContext(ctx, `
SELECT i.owner,
       i.table_name,
       i.index_name,
       i.index_type,
       i.uniqueness,
       c.column_name
FROM all_indexes i
LEFT JOIN all_ind_columns c
  ON c.index_owner = i.owner
 AND c.index_name = i.index_name
 AND c.table_owner = i.owner
 AND c.table_name = i.table_name
WHERE i.owner = :1
ORDER BY i.owner, i.table_name, i.index_name, c.column_position`, schema)
	if err != nil {
		return nil, fmt.Errorf("query oracle indexes: %w", err)
	}
	defer rows.Close()
	return scanOracleStyleIndexes(rows, "oracle")
}

func oracleForeignKeys(ctx context.Context, db *sql.DB, schema string) (map[string][]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ac.owner,
       acc.table_name,
       ac.constraint_name,
       acc.column_name,
       r.owner AS referenced_schema,
       r.table_name AS referenced_table,
       rcc.column_name AS referenced_column
FROM all_constraints ac
JOIN all_cons_columns acc
  ON acc.owner = ac.owner
 AND acc.constraint_name = ac.constraint_name
 AND acc.table_name = ac.table_name
JOIN all_constraints r
  ON r.owner = ac.r_owner
 AND r.constraint_name = ac.r_constraint_name
JOIN all_cons_columns rcc
  ON rcc.owner = r.owner
 AND rcc.constraint_name = r.constraint_name
 AND rcc.position = acc.position
WHERE ac.constraint_type = 'R'
  AND ac.owner = :1
ORDER BY ac.owner, acc.table_name, ac.constraint_name, acc.position`, schema)
	if err != nil {
		return nil, fmt.Errorf("query oracle foreign keys: %w", err)
	}
	defer rows.Close()
	return scanOracleStyleForeignKeys(rows, "oracle")
}
