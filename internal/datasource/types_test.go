package datasource

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	driver := fakeDriver{dbType: "mysql"}
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register driver: %v", err)
	}

	got, err := registry.Driver("MYSQL")
	if err != nil {
		t.Fatalf("get driver: %v", err)
	}
	if got.Type() != "mysql" {
		t.Fatalf("unexpected driver type: %s", got.Type())
	}

	if _, err := registry.Driver("postgresql"); !errors.Is(err, ErrDriverNotFound) {
		t.Fatalf("expected driver not found, got %v", err)
	}
}

func TestDefaultRegistryIncludesBuiltInDrivers(t *testing.T) {
	registry := DefaultRegistry()
	got := registry.Types()
	want := []string{"clickhouse", "dm8", "doris", "kingbase", "mysql", "oracle", "postgresql", "sqlserver"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected default drivers: got %v want %v", got, want)
	}
}

type fakeDriver struct {
	dbType string
}

func (d fakeDriver) Type() string {
	return d.dbType
}

func (d fakeDriver) Ping(ctx context.Context, cfg Config) error {
	return nil
}

func (d fakeDriver) Introspect(ctx context.Context, cfg Config) (*Metadata, error) {
	return &Metadata{}, nil
}

func (d fakeDriver) Query(ctx context.Context, cfg Config, sqlText string, maxRows int) (*QueryResult, error) {
	return &QueryResult{}, nil
}
