package datasource

import (
	"context"
	"database/sql"
	"fmt"
)

func NewPostgreSQLDriver() Driver {
	return newSQLQueryDriver("postgresql", "postgres", introspectPostgreSQL, postgresqlVersion)
}

func NewKingbaseDriver() Driver {
	return newSQLQueryDriver("kingbase", "postgres", introspectPostgreSQL, postgresqlVersion)
}

func postgresqlVersion(ctx context.Context, db *sql.DB) (string, error) {
	return readSingleString(ctx, db, "SELECT version()")
}

func introspectPostgreSQL(ctx context.Context, db *sql.DB) (*Metadata, error) {
	tables, err := postgresqlTables(ctx, db)
	if err != nil {
		return nil, err
	}
	columnsByTable, err := postgresqlColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	indexesByTable, err := postgresqlIndexes(ctx, db)
	if err != nil {
		return nil, err
	}
	foreignKeysByTable, err := postgresqlForeignKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	tables = attachColumns(tables, columnsByTable)
	tables = attachIndexes(tables, indexesByTable)
	tables = attachForeignKeys(tables, foreignKeysByTable)
	return buildMetadata(tables), nil
}

func postgresqlTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	rows, err := db.QueryContext(ctx, `
SELECT t.table_schema,
       t.table_name,
       t.table_type,
       COALESCE(obj_description(c.oid), '') AS table_comment,
       NULL::bigint AS table_rows
FROM information_schema.tables t
LEFT JOIN pg_catalog.pg_namespace n ON n.nspname = t.table_schema
LEFT JOIN pg_catalog.pg_class c ON c.relname = t.table_name AND c.relnamespace = n.oid
WHERE t.table_schema NOT IN ('pg_catalog', 'information_schema')
  AND t.table_type IN ('BASE TABLE', 'VIEW')
ORDER BY t.table_schema, t.table_name`)
	if err != nil {
		return nil, fmt.Errorf("query postgresql tables: %w", err)
	}
	defer rows.Close()
	return scanTableRows(rows, "postgresql")
}

func postgresqlColumns(ctx context.Context, db *sql.DB) (map[string][]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT c.table_schema,
       c.table_name,
       c.column_name,
       c.ordinal_position,
       c.data_type,
       c.udt_name,
       c.is_nullable,
       COALESCE(c.column_default, '') AS column_default,
       CASE WHEN pk.column_name IS NULL THEN '' ELSE 'PRI' END AS column_key,
       CASE WHEN fk.column_name IS NULL THEN '' ELSE 'FK' END AS foreign_key,
       COALESCE(pg_catalog.col_description(cls.oid, c.ordinal_position), '') AS column_comment
FROM information_schema.columns c
LEFT JOIN pg_catalog.pg_namespace n ON n.nspname = c.table_schema
LEFT JOIN pg_catalog.pg_class cls ON cls.relname = c.table_name AND cls.relnamespace = n.oid
LEFT JOIN (
  SELECT ku.table_schema, ku.table_name, ku.column_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.key_column_usage ku
    ON ku.constraint_name = tc.constraint_name
   AND ku.constraint_schema = tc.constraint_schema
   AND ku.table_schema = tc.table_schema
   AND ku.table_name = tc.table_name
  WHERE tc.constraint_type = 'PRIMARY KEY'
) pk ON pk.table_schema = c.table_schema
    AND pk.table_name = c.table_name
    AND pk.column_name = c.column_name
LEFT JOIN (
  SELECT ku.table_schema, ku.table_name, ku.column_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.key_column_usage ku
    ON ku.constraint_name = tc.constraint_name
   AND ku.constraint_schema = tc.constraint_schema
   AND ku.table_schema = tc.table_schema
   AND ku.table_name = tc.table_name
  WHERE tc.constraint_type = 'FOREIGN KEY'
) fk ON fk.table_schema = c.table_schema
    AND fk.table_name = c.table_name
    AND fk.column_name = c.column_name
WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY c.table_schema, c.table_name, c.ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("query postgresql columns: %w", err)
	}
	defer rows.Close()

	columnsByTable := map[string][]Column{}
	for rows.Next() {
		var tableSchema, tableName, columnName, dataType, columnType, nullable, columnKey, foreignKey string
		var ordinal int
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &columnName, &ordinal, &dataType, &columnType, &nullable, &defaultValue, &columnKey, &foreignKey, &comment); err != nil {
			return nil, fmt.Errorf("scan postgresql column: %w", err)
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
		return nil, fmt.Errorf("iterate postgresql columns: %w", err)
	}
	return columnsByTable, nil
}

func postgresqlIndexes(ctx context.Context, db *sql.DB) (map[string][]Index, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ns.nspname AS table_schema,
       tbl.relname AS table_name,
       idx.relname AS index_name,
       am.amname AS index_type,
       ix.indisunique AS unique_index,
       a.attname AS column_name
FROM pg_catalog.pg_index ix
JOIN pg_catalog.pg_class idx ON idx.oid = ix.indexrelid
JOIN pg_catalog.pg_class tbl ON tbl.oid = ix.indrelid
JOIN pg_catalog.pg_namespace ns ON ns.oid = tbl.relnamespace
JOIN pg_catalog.pg_am am ON am.oid = idx.relam
LEFT JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS keys(attnum, ord) ON true
LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = tbl.oid AND a.attnum = keys.attnum
WHERE ns.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY ns.nspname, tbl.relname, idx.relname, keys.ord`)
	if err != nil {
		return nil, fmt.Errorf("query postgresql indexes: %w", err)
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
			return nil, fmt.Errorf("scan postgresql index: %w", err)
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
		return nil, fmt.Errorf("iterate postgresql indexes: %w", err)
	}
	out := map[string][]Index{}
	for _, key := range order {
		out[tableKey(key.schema, key.table)] = append(out[tableKey(key.schema, key.table)], *grouped[key])
	}
	return out, nil
}

func postgresqlForeignKeys(ctx context.Context, db *sql.DB) (map[string][]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT tc.table_schema,
       tc.table_name,
       tc.constraint_name,
       kcu.column_name,
       ccu.table_schema AS referenced_schema,
       ccu.table_name AS referenced_table,
       ccu.column_name AS referenced_column
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = tc.constraint_name
 AND kcu.constraint_schema = tc.constraint_schema
 AND kcu.table_schema = tc.table_schema
 AND kcu.table_name = tc.table_name
JOIN information_schema.constraint_column_usage ccu
  ON ccu.constraint_name = tc.constraint_name
 AND ccu.constraint_schema = tc.constraint_schema
WHERE tc.constraint_type = 'FOREIGN KEY'
  AND tc.table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY tc.table_schema, tc.table_name, tc.constraint_name, kcu.ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("query postgresql foreign keys: %w", err)
	}
	defer rows.Close()

	out := map[string][]ForeignKey{}
	for rows.Next() {
		var fk ForeignKey
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName, &fk.ConstraintName, &fk.ColumnName, &fk.ReferencedSchema, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("scan postgresql foreign key: %w", err)
		}
		out[tableKey(schema, tableName)] = append(out[tableKey(schema, tableName)], fk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgresql foreign keys: %w", err)
	}
	return out, nil
}
