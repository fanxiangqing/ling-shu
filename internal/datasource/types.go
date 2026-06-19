package datasource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrDriverNotFound = errors.New("datasource driver not found")
	ErrInvalidConfig  = errors.New("invalid datasource config")
)

type Config struct {
	DSN string
}

type Versioner interface {
	Version(ctx context.Context, cfg Config) (string, error)
}

type Driver interface {
	Type() string
	Ping(ctx context.Context, cfg Config) error
	Introspect(ctx context.Context, cfg Config) (*Metadata, error)
	Query(ctx context.Context, cfg Config, sqlText string, maxRows int) (*QueryResult, error)
}

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

type Metadata struct {
	Version string
	Schemas []Schema
	Tables  []Table
}

type Schema struct {
	Name    string
	Comment string
}

type Table struct {
	Schema      string
	Name        string
	Type        string
	Comment     string
	RowCount    *int64
	Columns     []Column
	Indexes     []Index
	ForeignKeys []ForeignKey
}

type Column struct {
	Name            string
	OrdinalPosition int
	DataType        string
	ColumnType      string
	Nullable        bool
	DefaultValue    string
	IsPrimaryKey    bool
	IsForeignKey    bool
	Comment         string
}

type Index struct {
	Name    string
	Type    string
	Unique  bool
	Columns []string
}

type ForeignKey struct {
	ConstraintName   string
	ColumnName       string
	ReferencedSchema string
	ReferencedTable  string
	ReferencedColumn string
}

type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

func NewRegistry() *Registry {
	return &Registry{drivers: map[string]Driver{}}
}

func (r *Registry) Register(driver Driver) error {
	if driver == nil || strings.TrimSpace(driver.Type()) == "" {
		return ErrInvalidConfig
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[normalizeType(driver.Type())] = driver
	return nil
}

func (r *Registry) Driver(dbType string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	driver, ok := r.drivers[normalizeType(dbType)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, dbType)
	}
	return driver, nil
}

func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.drivers))
	for dbType := range r.drivers {
		types = append(types, dbType)
	}
	sort.Strings(types)
	return types
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(NewMySQLDriver())
	_ = registry.Register(NewPostgreSQLDriver())
	_ = registry.Register(NewKingbaseDriver())
	_ = registry.Register(NewSQLServerDriver())
	_ = registry.Register(NewOracleDriver())
	_ = registry.Register(NewDMDriver())
	_ = registry.Register(NewClickHouseDriver())
	_ = registry.Register(NewDorisDriver())
	return registry
}

func normalizeType(dbType string) string {
	return strings.ToLower(strings.TrimSpace(dbType))
}
