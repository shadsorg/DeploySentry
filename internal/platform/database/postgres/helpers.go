package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDB returns a *pgxpool.Pool connected to the integration-test Postgres
// instance. It reads DS_TEST_DATABASE_DSN from the environment, falling back
// to the standard local dev DSN used by `make dev-up` / `make migrate-up`.
// The pool is closed automatically when the test (and all its sub-tests) finish.
//
// These tests require a live Postgres reachable at the DSN above, so under
// `go test -short` (CI's unit-test step) we skip them rather than fail. The
// integration step opts in by running without -short and setting the env var.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB-backed test under -short; run without -short with a live Postgres")
	}
	dsn := os.Getenv("DS_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://deploysentry:deploysentry@localhost:5432/deploysentry?sslmode=disable&search_path=deploy"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("testDB: connect failed (%v) — skipping; set DS_TEST_DATABASE_DSN to a reachable Postgres to run", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("testDB: ping failed (%v) — skipping; set DS_TEST_DATABASE_DSN to a reachable Postgres to run", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// whereBuilder accumulates WHERE conditions and positional arguments for
// building dynamic SQL queries safely.
type whereBuilder struct {
	conditions []string
	args       []any
}

// Add appends a condition. The placeholder must use %d for the argument position,
// which will be replaced with the next $N placeholder.
// Example: w.Add("project_id = $%d", projectID)
func (w *whereBuilder) Add(condition string, arg any) {
	pos := len(w.args) + 1
	w.conditions = append(w.conditions, fmt.Sprintf(condition, pos))
	w.args = append(w.args, arg)
}

// Build returns the WHERE clause string and the accumulated arguments.
// Returns empty string and nil args if no conditions were added.
func (w *whereBuilder) Build() (string, []any) {
	if len(w.conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w.conditions, " AND "), w.args
}

// paginationClause returns a LIMIT/OFFSET clause and appends the args.
func paginationClause(limit, offset int, args []any) (string, []any) {
	if limit <= 0 {
		limit = 20
	}
	startPos := len(args) + 1
	clause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", startPos, startPos+1)
	args = append(args, limit, offset)
	return clause, args
}
