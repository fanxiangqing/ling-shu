package datasource

import (
	"context"
	"database/sql"
	"errors"
)

func NewDorisDriver() Driver {
	return newSQLQueryDriver("doris", "mysql", introspectMySQLCompatible, dorisVersion)
}

func dorisVersion(ctx context.Context, db *sql.DB) (string, error) {
	return readSingleString(ctx, db, "SELECT VERSION()")
}

func introspectMySQLCompatible(ctx context.Context, db *sql.DB) (*Metadata, error) {
	schema, err := currentDatabase(ctx, db)
	if err != nil {
		return nil, err
	}
	if schema == "" {
		return nil, errors.New("database name is required in dsn")
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
	tables = attachIndexes(tables, indexesByTable)
	tables = attachForeignKeys(tables, foreignKeysByTable)
	return &Metadata{
		Schemas: []Schema{{Name: schema}},
		Tables:  attachColumns(tables, columnsByTable),
	}, nil
}
