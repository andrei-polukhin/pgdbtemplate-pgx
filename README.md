# pgdbtemplate-pgx

[![Go Reference](https://pkg.go.dev/badge/github.com/andrei-polukhin/pgdbtemplate-pgx.svg)](https://pkg.go.dev/github.com/andrei-polukhin/pgdbtemplate-pgx)
[![CI](https://github.com/andrei-polukhin/pgdbtemplate-pgx/actions/workflows/test.yml/badge.svg)](https://github.com/andrei-polukhin/pgdbtemplate-pgx/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/andrei-polukhin/pgdbtemplate-pgx/blob/main/LICENSE)

A PostgreSQL connection provider for
[pgdbtemplate](https://github.com/andrei-polukhin/pgdbtemplate)
using the `pgx` driver with native connection pooling.

## Features

- **🔌 pgx driver** - Uses `jackc/pgx/v5` with native PostgreSQL protocol
- **🔒 Thread-safe** - concurrent connection management with connection pooling
- **⚙️ Advanced connection pooling** - Built-in pool management with configurable limits
- **🎯 PostgreSQL-native** - Full PostgreSQL type support and features
- **🧪 Test-ready** - designed for high-performance test database creation
- **📦 Compatible** with pgdbtemplate's template database workflow

## Installation

```bash
go get github.com/andrei-polukhin/pgdbtemplate-pgx
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/andrei-polukhin/pgdbtemplate"
	pgdbtemplatepgx "github.com/andrei-polukhin/pgdbtemplate-pgx"
)

func main() {
	// Create a connection provider with pooling options.
	connStringFunc := func(dbName string) string {
		return fmt.Sprintf("postgres://user:pass@localhost/%s", dbName)
	}
	provider := pgdbtemplatepgx.NewConnectionProvider(
		connStringFunc,
		pgdbtemplatepgx.WithMaxConns(25),
		pgdbtemplatepgx.WithMinConns(5),
	)

	// Create migration runner.
	migrationRunner := pgdbtemplate.NewFileMigrationRunner(
		[]string{"./migrations"},
		pgdbtemplate.AlphabeticalMigrationFilesSorting,
	)

	// Create template manager.
	config := pgdbtemplate.Config{
		ConnectionProvider: provider,
		MigrationRunner:    migrationRunner,
	}

	tm, err := pgdbtemplate.NewTemplateManager(config)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize template with migrations.
	ctx := context.Background()
	if err := tm.Initialize(ctx); err != nil {
		log.Fatal(err)
	}

	// Create test database (fast!).
	testDB, testDBName, err := tm.CreateTestDatabase(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer testDB.Close()
	defer tm.DropTestDatabase(ctx, testDBName)

	// Use testDB for testing...
	log.Printf("Test database %s ready!", testDBName)
}
```

## Usage Examples

### 1. Basic Testing with pgx

```go
package myapp_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/andrei-polukhin/pgdbtemplate"
	pgdbtemplatepgx "github.com/andrei-polukhin/pgdbtemplate-pgx"
)

var templateManager *pgdbtemplate.TemplateManager

func TestMain(m *testing.M) {
	// Setup template manager once.
	if err := setupTemplateManager(); err != nil {
		log.Fatalf("failed to setup template manager: %v", err)
	}

	// Run tests.
	code := m.Run()

	// Cleanup.
	templateManager.Cleanup(context.Background())
	os.Exit(code)
}

func setupTemplateManager() error {
	baseConnString := "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"

	// Create pgx connection provider with connection pooling.
	connStringFunc := func(dbName string) string {
		return pgdbtemplate.ReplaceDatabaseInConnectionString(baseConnString, dbName)
	}

	provider := pgdbtemplatepgx.NewConnectionProvider(
		connStringFunc,
		pgdbtemplatepgx.WithMaxConns(10),
		pgdbtemplatepgx.WithMinConns(2),
	)

	// Create migration runner.
	migrationRunner := pgdbtemplate.NewFileMigrationRunner(
		[]string{"./testdata/migrations"},
		pgdbtemplate.AlphabeticalMigrationFilesSorting,
	)

	// Configure template manager.
	config := pgdbtemplate.Config{
		ConnectionProvider: provider,
		MigrationRunner:    migrationRunner,
	}

	var err error
	templateManager, err = pgdbtemplate.NewTemplateManager(config)
	if err != nil {
		return fmt.Errorf("failed to create template manager: %w", err)
	}

	// Initialize template database with migrations.
	ctx := context.Background()
	if err := templateManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize template manager: %w", err)
	}

	return nil
}

func TestUserCreation(t *testing.T) {
	ctx := context.Background()

	// Create test database from template.
	testDB, testDBName, err := templateManager.CreateTestDatabase(ctx)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Test your application logic here...
	var count int
	row := testDB.QueryRowContext(ctx, "SELECT 1")
	if err := row.Scan(&count); err != nil {
		t.Errorf("failed to query: %v", err)
	}
}
```

### 2. Advanced Connection Pooling

```go
// Configure connection provider with custom pool configuration.
provider := pgdbtemplatepgx.NewConnectionProvider(
	connStringFunc,
	pgdbtemplatepgx.WithMaxConns(50),        // Maximum connections.
	pgdbtemplatepgx.WithMinConns(10),        // Minimum connections.
)

// Or use a complete pool configuration.
poolConfig := pgxpool.Config{}
poolConfig.MaxConns = 100
poolConfig.MinConns = 5
poolConfig.MaxConnLifetime = time.Hour

provider := pgdbtemplatepgx.NewConnectionProvider(
	connStringFunc,
	pgdbtemplatepgx.WithPoolConfig(poolConfig),
)
```

## Thread Safety

The `ConnectionProvider` is thread-safe and can be used concurrently
from multiple goroutines. Connection pools are shared across multiple
`Connect` calls to the same database.

## Best Practices

- Use connection pooling options appropriate for your test load
- Set `POSTGRES_CONNECTION_STRING` environment variable for tests
- Close connections and drop test databases after use
- Use context timeouts for connection operations
- Configure MinConns > 0 for better performance in concurrent scenarios

## Performance Benefits

The pgx implementation provides excellent performance through:

- **Native PostgreSQL protocol** - Direct binary protocol communication
- **Built-in connection pooling** - Efficient connection reuse
- **Advanced type support** - Native PostgreSQL type handling
- **Optimized for concurrency** - Designed for high-throughput applications

## License

MIT License - see [LICENSE](LICENSE) file for details.
