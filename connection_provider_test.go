package pgdbtemplatepgx_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrei-polukhin/pgdbtemplate"
	pgdbtemplatepgx "github.com/andrei-polukhin/pgdbtemplate-pgx"
)

// testConnectionStringFuncPgx creates a connection string for pgx tests.
func testConnectionStringFuncPgx(dbName string) string {
	return pgdbtemplate.ReplaceDatabaseInConnectionString(testConnectionString, dbName)
}

func TestPgxConnectionProvider(t *testing.T) {
	t.Parallel()
	c := qt.New(t)
	ctx := context.Background()

	c.Run("Basic pgx connection", func(c *qt.C) {
		c.Parallel()
		provider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)
		defer provider.Close()

		conn, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn.Close(), qt.IsNil) }()

		// Verify the connection works.
		var value int
		row := conn.QueryRowContext(ctx, "SELECT 1")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 1)
	})

	c.Run("Pgx connection with pool options", func(c *qt.C) {
		c.Parallel()
		provider := pgdbtemplatepgx.NewConnectionProvider(
			testConnectionStringFuncPgx,
			pgdbtemplatepgx.WithMaxConns(5),
			pgdbtemplatepgx.WithMinConns(1),
		)
		defer provider.Close()

		conn, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn.Close(), qt.IsNil) }()

		// Verify the connection works.
		var value int
		row := conn.QueryRowContext(ctx, "SELECT 1")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 1)
	})

	c.Run("MinConns option alone", func(c *qt.C) {
		c.Parallel()
		// Test WithPgxMinConns when poolConfig is initially nil.
		provider := pgdbtemplatepgx.NewConnectionProvider(
			testConnectionStringFuncPgx,
			pgdbtemplatepgx.WithMaxConns(2),
		)
		defer provider.Close()

		conn, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn.Close(), qt.IsNil) }()

		// Verify the connection works.
		var value int
		row := conn.QueryRowContext(ctx, "SELECT 1")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 1)
	})

	c.Run("Custom pool configuration", func(c *qt.C) {
		c.Parallel()
		// Create a custom pool config.
		baseConnString := testConnectionStringFuncPgx("postgres")
		poolConfig, err := pgxpool.ParseConfig(baseConnString)
		c.Assert(err, qt.IsNil)
		poolConfig.MaxConns = 3
		poolConfig.MinConns = 1

		provider := pgdbtemplatepgx.NewConnectionProvider(
			testConnectionStringFuncPgx,
			pgdbtemplatepgx.WithPoolConfig(*poolConfig),
		)
		defer provider.Close()

		conn, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn.Close(), qt.IsNil) }()

		// Verify the connection works.
		var value int
		row := conn.QueryRowContext(ctx, "SELECT 1")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 1)
	})

	c.Run("Connection error handling", func(c *qt.C) {
		// Test with invalid connection string.
		invalidConnStringFunc := func(dbName string) string {
			return "invalid://connection/string"
		}
		provider := pgdbtemplatepgx.NewConnectionProvider(invalidConnStringFunc)
		defer provider.Close()

		_, err := provider.Connect(ctx, "testdb")
		c.Assert(err, qt.ErrorMatches, "failed to parse connection string:.*")
	})

	c.Run("Connection to nonexistent database", func(c *qt.C) {
		c.Parallel()
		nonExistentFunc := func(dbName string) string {
			return pgdbtemplate.ReplaceDatabaseInConnectionString(testConnectionString, "nonexistent_db_12345")
		}
		provider := pgdbtemplatepgx.NewConnectionProvider(nonExistentFunc)
		defer provider.Close()

		conn, err := provider.Connect(ctx, "nonexistent")
		c.Assert(err, qt.IsNotNil)
		c.Assert(conn, qt.IsNil)
		c.Assert(err, qt.ErrorMatches, "failed to ping database:.*")
	})

	c.Run("Pool reuse", func(c *qt.C) {
		c.Parallel()
		provider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)
		defer provider.Close()

		// Connect to the same database twice to test pool reuse.
		conn1, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn1.Close(), qt.IsNil) }()

		conn2, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.IsNil)
		defer func() { c.Assert(conn2.Close(), qt.IsNil) }()

		// Both connections should work.
		var value int
		row := conn1.QueryRowContext(ctx, "SELECT 1")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 1)

		row = conn2.QueryRowContext(ctx, "SELECT 2")
		err = row.Scan(&value)
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 2)
	})

	c.Run("Concurrent pool double-check", func(c *qt.C) {
		c.Parallel()
		provider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)
		defer provider.Close()

		ctx := context.Background()
		dbName := "postgres"

		start := make(chan struct{})
		results := make(chan error, 2)
		openPoolConn := func() {
			<-start // Wait for the signal to start.
			conn, err := provider.Connect(ctx, dbName)
			if conn != nil {
				defer conn.Close()
			}
			results <- err
		}
		go openPoolConn()
		go openPoolConn()

		// Signal both goroutines to start simultaneously.
		close(start)

		// Wait for both goroutines to finish.
		// Both should succeed without error.
		// This tests the double-check locking in GetPool.
		for i := 0; i < 2; i++ {
			err := <-results
			c.Assert(err, qt.IsNil)
		}
	})

	c.Run("Wrong MaxConns handling", func(c *qt.C) {
		provider := pgdbtemplatepgx.NewConnectionProvider(
			testConnectionStringFuncPgx,
			pgdbtemplatepgx.WithMaxConns(-1), // Pool will not be created.
		)
		defer provider.Close()

		_, err := provider.Connect(ctx, "postgres")
		c.Assert(err, qt.ErrorMatches, "failed to create connection pool:.*")
	})

	c.Run("Context cancellation during pool creation", func(c *qt.C) {
		// Create a context that gets cancelled immediately.
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		provider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)
		defer provider.Close()

		_, err := provider.Connect(cancelCtx, "postgres")
		c.Assert(err, qt.ErrorMatches, "failed to ping database:.*")
	})
}

func TestTemplateManagerWithPgx(t *testing.T) {
	t.Parallel()
	c := qt.New(t)
	ctx := context.Background()

	// Create pgx connection provider.
	provider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)

	// Create migration runner.
	migrationRunner := createTestMigrationRunner(c)

	templateName := fmt.Sprintf("pgx_test_template_%d_%d", time.Now().UnixNano(), os.Getpid())
	config := pgdbtemplate.Config{
		ConnectionProvider: provider,
		MigrationRunner:    migrationRunner,
		TemplateName:       templateName,
		TestDBPrefix:       fmt.Sprintf("pgx_test_%d_%d", time.Now().UnixNano(), os.Getpid()),
	}

	tm, err := pgdbtemplate.NewTemplateManager(config)
	c.Assert(err, qt.IsNil)

	// Initialize template.
	err = tm.Initialize(ctx)
	c.Assert(err, qt.IsNil)

	// CRITICAL: Close all pgx connections to allow template database to be used as a template.
	// pgx keeps connections in pools which prevents PostgreSQL from using the template database.
	provider.Close()

	// Create a new provider instance for test database operations.
	testProvider := pgdbtemplatepgx.NewConnectionProvider(testConnectionStringFuncPgx)

	// Update the template manager with the new provider.
	config.ConnectionProvider = testProvider
	testTM, err := pgdbtemplate.NewTemplateManager(config)
	c.Assert(err, qt.IsNil)

	// Create test database.
	testDB, testDBName, err := testTM.CreateTestDatabase(ctx)
	c.Assert(err, qt.IsNil)
	defer func() {
		c.Assert(testDB.Close(), qt.IsNil)
		c.Assert(testTM.DropTestDatabase(ctx, testDBName), qt.IsNil)
		// Close all connections before cleanup.
		testProvider.Close()
		c.Assert(testTM.Cleanup(ctx), qt.IsNil)
	}()

	// Verify test database has the migrated schema.
	var count int
	row := testDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table")
	err = row.Scan(&count)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2) // Should have 2 rows from migration.

	// Test pgx-specific functionality.
	result, err := testDB.ExecContext(ctx, "INSERT INTO test_table (name) VALUES ($1)", "pgx_test")
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	// Verify the insertion.
	row = testDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table")
	err = row.Scan(&count)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3) // Should now have 3 rows.
}

// Helper function to create a test migration runner.
func createTestMigrationRunner(c *qt.C) pgdbtemplate.MigrationRunner {
	// Create temporary migration files.
	tempDir := c.TempDir()

	migration1 := `
	CREATE TABLE test_table (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);`

	migration2 := `
	INSERT INTO test_table (name)
	VALUES ('test_data_1'), ('test_data_2');`

	migration1Path := tempDir + "/001_create_table.sql"
	migration2Path := tempDir + "/002_insert_data.sql"

	err := os.WriteFile(migration1Path, []byte(migration1), 0644)
	c.Assert(err, qt.IsNil)

	err = os.WriteFile(migration2Path, []byte(migration2), 0644)
	c.Assert(err, qt.IsNil)

	return pgdbtemplate.NewFileMigrationRunner([]string{tempDir}, pgdbtemplate.AlphabeticalMigrationFilesSorting)
}
