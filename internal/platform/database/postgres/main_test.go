package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMain runs after every test in this package and sweeps rollout rows that
// test helpers insert against the shared dev database. Without this, running
// `go test ./internal/platform/database/postgres/...` leaves orphan rollouts
// in the `deploy` schema (strategy_snapshot.name = 'test' | 's',
// created_by IS NULL), because target_ref is JSONB and has no FK back to
// deployments. Those orphans then show up as live "pending" rollouts in the
// RolloutsPage and are referenced by IDs that don't exist in `deployments`,
// making the DeployHistory screen look empty next to a busy Rollouts screen.
//
// The sweep runs only if a dev DB is reachable; skipping a cleanup is fine —
// the tests themselves skip when the DSN is unreachable.
func TestMain(m *testing.M) {
	code := m.Run()

	dsn := os.Getenv("DS_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://deploysentry:deploysentry@localhost:5432/deploysentry?sslmode=disable&search_path=deploy"
	}
	if err := sweepTestRollouts(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "postgres tests: cleanup sweep failed: %v\n", err)
	}

	os.Exit(code)
}

func sweepTestRollouts(dsn string) error {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	// Match only fixtures: created_by IS NULL restricts the sweep to rows
	// the test helpers inserted. Real rollouts set created_by via the
	// attacher. Cascading FKs on rollout_phases / rollout_events /
	// rollout_group_members handle their cleanup automatically.
	const q = `
		DELETE FROM rollouts
		WHERE created_by IS NULL
		  AND strategy_snapshot->>'name' IN ('test', 's')`
	_, err = pool.Exec(ctx, q)
	return err
}
