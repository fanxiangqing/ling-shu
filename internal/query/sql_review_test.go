package query

import "testing"

func TestSQLReviewerBlocksWriteSQL(t *testing.T) {
	reviewer := NewSQLReviewer(200, 1000)
	result := reviewer.Review("delete from orders")
	if result.Passed {
		t.Fatal("expected write SQL to be blocked")
	}
	if result.BlockedReason == "" {
		t.Fatal("expected blocked reason")
	}
}

func TestSQLReviewerAllowsSelectAndAddsLimit(t *testing.T) {
	reviewer := NewSQLReviewer(200, 1000)
	result := reviewer.Review("select * from orders")
	if !result.Passed {
		t.Fatalf("expected select SQL to pass: %s", result.BlockedReason)
	}
	if result.NormalizedSQL != "select * from orders LIMIT 200" {
		t.Fatalf("unexpected normalized sql: %s", result.NormalizedSQL)
	}
}

func TestSQLReviewerAddsOracleFetchLimit(t *testing.T) {
	reviewer := NewSQLReviewer(200, 1000)
	result := reviewer.ReviewWithDialect("select * from orders", 50, "oracle")
	if !result.Passed {
		t.Fatalf("expected oracle select SQL to pass: %s", result.BlockedReason)
	}
	if result.NormalizedSQL != "select * from orders FETCH FIRST 50 ROWS ONLY" {
		t.Fatalf("unexpected oracle normalized sql: %s", result.NormalizedSQL)
	}
}

func TestSQLReviewerAddsSQLServerTopLimit(t *testing.T) {
	reviewer := NewSQLReviewer(200, 1000)
	result := reviewer.ReviewWithDialect("select distinct name from users", 20, "sqlserver")
	if !result.Passed {
		t.Fatalf("expected sqlserver select SQL to pass: %s", result.BlockedReason)
	}
	if result.NormalizedSQL != "SELECT DISTINCT TOP 20 name from users" {
		t.Fatalf("unexpected sqlserver normalized sql: %s", result.NormalizedSQL)
	}
}

func TestSQLReviewerKeepsComplexSQLServerWithQuery(t *testing.T) {
	reviewer := NewSQLReviewer(200, 1000)
	result := reviewer.ReviewWithDialect("with t as (select id from users) select * from t", 20, "sqlserver")
	if !result.Passed {
		t.Fatalf("expected sqlserver with SQL to pass: %s", result.BlockedReason)
	}
	if result.NormalizedSQL != "with t as (select id from users) select * from t" {
		t.Fatalf("unexpected sqlserver normalized sql: %s", result.NormalizedSQL)
	}
}
