package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the local docker-compose Postgres (see
// docker-compose.yml) and returns a pool scoped to a fresh, empty database
// created for this test alone, so migration tests never interfere with each
// other or with a schema left over from a previous run.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
	if err != nil {
		t.Skipf("skipping: cannot reach local postgres (is `docker compose up -d` running?): %v", err)
	}
	defer adminPool.Close()
	if err := adminPool.Ping(ctx); err != nil {
		t.Skipf("skipping: cannot reach local postgres (is `docker compose up -d` running?): %v", err)
	}

	dbName := "tb_test_" + randomSuffix()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanupPool, err := pgxpool.New(cleanupCtx, "postgres://tallybook:tallybook@localhost:5433/tallybook?sslmode=disable")
		if err != nil {
			return
		}
		defer cleanupPool.Close()
		_, _ = cleanupPool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+dbName)
	})

	pool, err := pgxpool.New(ctx, "postgres://tallybook:tallybook@localhost:5433/"+dbName+"?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test database %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func randomSuffix() string {
	// Good enough for test-database naming: monotonic-ish and unique per
	// process without pulling in a UUID dependency. Digits only — the name
	// is interpolated directly into CREATE/DROP DATABASE, which cannot be
	// parameterized, so it must already be a safe bare identifier.
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// FuzzParseMigrations exercises ParseMigrations with arbitrary in-memory
// filesystems and asserts two invariants that hold regardless of input:
//   - the function never panics;
//   - when it succeeds, the returned slice is in strictly ascending version order.
//
// The seed corpus covers the boundary cases the issue asks for: a valid
// sorted set, an empty filesystem (must yield ErrNoMigrations), a bad
// filename, duplicate versions, and zero-padded filenames (0010 must sort
// after 0009, never before it).
func FuzzParseMigrations(f *testing.F) {
	f.Add("0001_first.sql\x000002_second.sql\x000010_tenth.sql")
	f.Add("") // empty → ErrNoMigrations
	f.Add("not-numbered.sql")
	f.Add("0001_x.sql\x000001_y.sql") // duplicate version
	f.Add("0009_nine.sql\x000010_ten.sql")
	f.Add("0001_a.sql\x000003_c.sql") // version gap: gap is not an error

	f.Fuzz(func(t *testing.T, names string) {
		fsys := fstest.MapFS{}
		for _, name := range strings.Split(names, "\x00") {
			if name != "" {
				fsys[name] = &fstest.MapFile{Data: []byte("SELECT 1;")}
			}
		}

		migrations, err := ParseMigrations(fsys)
		if err != nil {
			if len(fsys) == 0 && !errors.Is(err, ErrNoMigrations) {
				t.Errorf("empty filesystem: got %v, want errors.Is(err, ErrNoMigrations)", err)
			}
			return
		}

		for i := 1; i < len(migrations); i++ {
			if migrations[i].Version <= migrations[i-1].Version {
				t.Fatalf(
					"migrations not in ascending version order: [%d] version=%d, [%d] version=%d",
					i-1, migrations[i-1].Version, i, migrations[i].Version,
				)
			}
		}
	})
}

func TestParseMigrations_SortsByVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql": {Data: []byte("SELECT 2;")},
		"0001_first.sql":  {Data: []byte("SELECT 1;")},
		"0010_tenth.sql":  {Data: []byte("SELECT 10;")},
	}

	migrations, err := ParseMigrations(fsys)
	if err != nil {
		t.Fatalf("ParseMigrations returned unexpected error: %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("got %d migrations, want 3", len(migrations))
	}
	wantOrder := []int{1, 2, 10}
	for i, want := range wantOrder {
		if migrations[i].Version != want {
			t.Errorf("migrations[%d].Version = %d, want %d", i, migrations[i].Version, want)
		}
	}
	if migrations[0].Name != "first" {
		t.Errorf("migrations[0].Name = %q, want %q", migrations[0].Name, "first")
	}
}

func TestParseMigrations_RejectsBadFilename(t *testing.T) {
	fsys := fstest.MapFS{
		"not-numbered.sql": {Data: []byte("SELECT 1;")},
	}
	_, err := ParseMigrations(fsys)
	if err == nil {
		t.Fatal("ParseMigrations returned nil error for a malformed filename")
	}
}

func TestParseMigrations_RejectsDuplicateVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_first.sql":      {Data: []byte("SELECT 1;")},
		"0001_also_first.sql": {Data: []byte("SELECT 2;")},
	}
	_, err := ParseMigrations(fsys)
	if err == nil {
		t.Fatal("ParseMigrations returned nil error for a duplicate version number")
	}
}

func TestParseMigrations_EmptyDirReturnsErrNoMigrations(t *testing.T) {
	_, err := ParseMigrations(fstest.MapFS{})
	if !errors.Is(err, ErrNoMigrations) {
		t.Fatalf("ParseMigrations error = %v, want ErrNoMigrations", err)
	}
}

func TestMigrate_AppliesInOrderAndIsIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	migrations := []Migration{
		{Version: 1, Name: "create_widgets", Filename: "0001_create_widgets.sql",
			SQL: `CREATE TABLE widgets (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`},
		{Version: 2, Name: "add_widget_color", Filename: "0002_add_widget_color.sql",
			SQL: `ALTER TABLE widgets ADD COLUMN color TEXT NOT NULL DEFAULT 'red'`},
	}

	applied, err := Migrate(ctx, pool, migrations)
	if err != nil {
		t.Fatalf("Migrate returned unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("Migrate applied %d migrations, want 2", len(applied))
	}

	// The schema change actually happened.
	var color string
	if err := pool.QueryRow(ctx,
		`INSERT INTO widgets (name) VALUES ('gear') RETURNING color`,
	).Scan(&color); err != nil {
		t.Fatalf("insert into widgets: %v", err)
	}
	if color != "red" {
		t.Errorf("color = %q, want %q", color, "red")
	}

	// Running again applies nothing and does not error, even though the
	// migrations would fail if re-executed (CREATE TABLE would collide).
	applied, err = Migrate(ctx, pool, migrations)
	if err != nil {
		t.Fatalf("Migrate returned unexpected error on second run: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("Migrate re-applied %d migrations, want 0", len(applied))
	}
}

func TestMigrate_StopsAtFirstFailureAndLeavesPriorMigrationsApplied(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	migrations := []Migration{
		{Version: 1, Name: "create_gadgets", Filename: "0001_create_gadgets.sql",
			SQL: `CREATE TABLE gadgets (id BIGSERIAL PRIMARY KEY)`},
		{Version: 2, Name: "broken", Filename: "0002_broken.sql",
			SQL: `THIS IS NOT VALID SQL`},
		{Version: 3, Name: "create_gizmos", Filename: "0003_create_gizmos.sql",
			SQL: `CREATE TABLE gizmos (id BIGSERIAL PRIMARY KEY)`},
	}

	applied, err := Migrate(ctx, pool, migrations)
	if err == nil {
		t.Fatal("Migrate returned nil error for a batch containing invalid SQL")
	}
	if len(applied) != 1 || applied[0].Version != 1 {
		t.Fatalf("Migrate applied %v before failing, want only version 1", applied)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gadgets')`,
	).Scan(&exists); err != nil {
		t.Fatalf("check gadgets table: %v", err)
	}
	if !exists {
		t.Error("gadgets table does not exist, but its migration should have committed before the failure")
	}

	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gizmos')`,
	).Scan(&exists); err != nil {
		t.Fatalf("check gizmos table: %v", err)
	}
	if exists {
		t.Error("gizmos table exists, but its migration comes after the failed one and should not have run")
	}

	// Fixing the broken migration and re-running picks up where it left off.
	migrations[1].SQL = `CREATE TABLE fixed (id BIGSERIAL PRIMARY KEY)`
	applied, err = Migrate(ctx, pool, migrations)
	if err != nil {
		t.Fatalf("Migrate returned unexpected error after fixing the migration: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("Migrate applied %d migrations on retry, want 2 (versions 2 and 3)", len(applied))
	}
}

func TestMigrate_NoMigrationsIsANoop(t *testing.T) {
	pool := testPool(t)
	applied, err := Migrate(context.Background(), pool, nil)
	if err != nil {
		t.Fatalf("Migrate returned unexpected error for an empty migration set: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("Migrate applied %d migrations from an empty set, want 0", len(applied))
	}
}

func TestMigrate_ConcurrentStartups(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	migrations := []Migration{{Version: 1, Name: "concurrent", Filename: "0001_concurrent.sql",
		SQL: `SELECT pg_sleep(0.1); CREATE TABLE concurrent_widgets (id INTEGER PRIMARY KEY)`}}
	type outcome struct {
		applied []Migration
		err     error
	}
	results := make(chan outcome, 4)
	start := make(chan struct{})
	for range 4 {
		go func() {
			<-start
			applied, err := Migrate(ctx, pool, migrations)
			results <- outcome{applied, err}
		}()
	}
	close(start)
	total := 0
	for range 4 {
		result := <-results
		if result.err != nil {
			t.Errorf("concurrent migration: %v", result.err)
		}
		total += len(result.applied)
	}
	if total != 1 {
		t.Errorf("applied %d migrations across startups, want 1", total)
	}
}
