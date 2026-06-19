package datasource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type metadataLoader func(ctx context.Context, db *sql.DB) (*Metadata, error)
type versionLoader func(ctx context.Context, db *sql.DB) (string, error)

type sqlQueryDriver struct {
	dbType        string
	driverName    string
	loader        metadataLoader
	versionLoader versionLoader
}

func newSQLQueryDriver(dbType string, driverName string, loader metadataLoader, loaderVersion versionLoader) *sqlQueryDriver {
	return &sqlQueryDriver{
		dbType:        dbType,
		driverName:    driverName,
		loader:        loader,
		versionLoader: loaderVersion,
	}
}

func (d *sqlQueryDriver) Type() string {
	return d.dbType
}

func (d *sqlQueryDriver) Ping(ctx context.Context, cfg Config) error {
	db, err := openSQLDatabase(d.driverName, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (d *sqlQueryDriver) Version(ctx context.Context, cfg Config) (string, error) {
	if d.versionLoader == nil {
		return "", nil
	}
	db, err := openSQLDatabase(d.driverName, cfg)
	if err != nil {
		return "", err
	}
	defer db.Close()
	return d.versionLoader(ctx, db)
}

func (d *sqlQueryDriver) Introspect(ctx context.Context, cfg Config) (*Metadata, error) {
	if d.loader == nil {
		return nil, fmt.Errorf("%w: %s introspect loader is empty", ErrInvalidConfig, d.dbType)
	}
	db, err := openSQLDatabase(d.driverName, cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	metadata, err := d.loader(ctx, db)
	if err != nil {
		return nil, err
	}
	if d.versionLoader != nil {
		if version, versionErr := d.versionLoader(ctx, db); versionErr == nil {
			metadata.Version = version
		}
	}
	return metadata, nil
}

func (d *sqlQueryDriver) Query(ctx context.Context, cfg Config, sqlText string, maxRows int) (*QueryResult, error) {
	db, err := openSQLDatabase(d.driverName, cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return querySQLRows(ctx, db, d.dbType, sqlText, maxRows)
}

func openSQLDatabase(driverName string, cfg Config) (*sql.DB, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, errors.New("dsn is required")
	}
	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open %s datasource: %w", driverName, err)
	}
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	return db, nil
}

func querySQLRows(ctx context.Context, db *sql.DB, dbType string, sqlText string, maxRows int) (*QueryResult, error) {
	if maxRows <= 0 {
		maxRows = 200
	}
	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("query %s datasource: %w", dbType, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read %s result columns: %w", dbType, err)
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
			return nil, fmt.Errorf("scan %s result row: %w", dbType, err)
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeSQLValue(values[i])
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s result rows: %w", dbType, err)
	}
	return result, nil
}

func buildMetadata(tables []Table) *Metadata {
	schemaMap := make(map[string]Schema)
	for _, table := range tables {
		if table.Schema == "" {
			continue
		}
		if _, ok := schemaMap[table.Schema]; !ok {
			schemaMap[table.Schema] = Schema{Name: table.Schema}
		}
	}
	schemas := make([]Schema, 0, len(schemaMap))
	for _, schema := range schemaMap {
		schemas = append(schemas, schema)
	}
	return &Metadata{Schemas: schemas, Tables: tables}
}

func attachColumns(tables []Table, columnsByTable map[string][]Column) []Table {
	for i := range tables {
		key := tableKey(tables[i].Schema, tables[i].Name)
		tables[i].Columns = columnsByTable[key]
	}
	return tables
}

func attachIndexes(tables []Table, indexesByTable map[string][]Index) []Table {
	for i := range tables {
		key := tableKey(tables[i].Schema, tables[i].Name)
		tables[i].Indexes = indexesByTable[key]
	}
	return tables
}

func attachForeignKeys(tables []Table, foreignKeysByTable map[string][]ForeignKey) []Table {
	for i := range tables {
		key := tableKey(tables[i].Schema, tables[i].Name)
		tables[i].ForeignKeys = foreignKeysByTable[key]
	}
	return tables
}

func scanTableRows(rows *sql.Rows, dbType string) ([]Table, error) {
	var tables []Table
	for rows.Next() {
		var tableSchema, tableName, tableType string
		var comment sql.NullString
		var rowCount sql.NullInt64
		if err := rows.Scan(&tableSchema, &tableName, &tableType, &comment, &rowCount); err != nil {
			return nil, fmt.Errorf("scan %s table: %w", dbType, err)
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
		return nil, fmt.Errorf("iterate %s tables: %w", dbType, err)
	}
	return tables, nil
}

func scanOracleStyleIndexes(rows *sql.Rows, dbType string) (map[string][]Index, error) {
	type indexKey struct {
		schema string
		table  string
		name   string
	}
	grouped := map[indexKey]*Index{}
	order := make([]indexKey, 0)
	for rows.Next() {
		var schema, tableName, indexName string
		var indexType, uniqueness, columnName sql.NullString
		if err := rows.Scan(&schema, &tableName, &indexName, &indexType, &uniqueness, &columnName); err != nil {
			return nil, fmt.Errorf("scan %s index: %w", dbType, err)
		}
		key := indexKey{schema: schema, table: tableName, name: indexName}
		index, ok := grouped[key]
		if !ok {
			index = &Index{
				Name:   indexName,
				Type:   indexType.String,
				Unique: strings.EqualFold(uniqueness.String, "UNIQUE"),
			}
			grouped[key] = index
			order = append(order, key)
		}
		if columnName.Valid && strings.TrimSpace(columnName.String) != "" {
			index.Columns = append(index.Columns, columnName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s indexes: %w", dbType, err)
	}
	out := map[string][]Index{}
	for _, key := range order {
		out[tableKey(key.schema, key.table)] = append(out[tableKey(key.schema, key.table)], *grouped[key])
	}
	return out, nil
}

func scanOracleStyleForeignKeys(rows *sql.Rows, dbType string) (map[string][]ForeignKey, error) {
	out := map[string][]ForeignKey{}
	for rows.Next() {
		var schema, tableName string
		var fk ForeignKey
		if err := rows.Scan(&schema, &tableName, &fk.ConstraintName, &fk.ColumnName, &fk.ReferencedSchema, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, fmt.Errorf("scan %s foreign key: %w", dbType, err)
		}
		out[tableKey(schema, tableName)] = append(out[tableKey(schema, tableName)], fk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s foreign keys: %w", dbType, err)
	}
	return out, nil
}

func readSingleString(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	var value sql.NullString
	if err := db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		return "", err
	}
	return strings.TrimSpace(value.String), nil
}
