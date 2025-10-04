package pgdbtemplatepgx

import (
	"context"
	"fmt"
	"sync"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionProvider implements pgdbtemplate.ConnectionProvider
// using pgx driver with connection pooling.
type ConnectionProvider struct {
	connectionStringFunc func(string) string
	poolConfig           pgxpool.Config

	mu    sync.RWMutex
	pools map[string]*pgxpool.Pool
}

// NewConnectionProvider creates a new pgx-based connection provider.
func NewConnectionProvider(connectionStringFunc func(string) string, opts ...ConnectionOption) *ConnectionProvider {
	provider := &ConnectionProvider{
		connectionStringFunc: connectionStringFunc,
		pools:                make(map[string]*pgxpool.Pool),
	}

	for _, opt := range opts {
		opt(provider)
	}
	return provider
}

// Connect implements pgdbtemplate.ConnectionProvider.Connect.
func (p *ConnectionProvider) Connect(ctx context.Context, databaseName string) (pgdbtemplate.DatabaseConnection, error) {
	// Check if we already have a pool for this database.
	p.mu.RLock()
	if pool, exists := p.pools[databaseName]; exists {
		p.mu.RUnlock()
		return &DatabaseConnection{
			Pool:     pool,
			provider: p,
			dbName:   databaseName,
		}, nil
	}
	p.mu.RUnlock()

	// Create new pool.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if pool, exists := p.pools[databaseName]; exists {
		return &DatabaseConnection{Pool: pool, provider: p, dbName: databaseName}, nil
	}

	// Parse connection string first.
	connString := p.connectionStringFunc(databaseName)
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Apply pool configuration settings if provided.
	// MaxConns must be checked (pgx validates >= 1).
	if p.poolConfig.MaxConns != 0 {
		config.MaxConns = p.poolConfig.MaxConns
	}
	// These could be set directly (0 is safe).
	config.MinConns = p.poolConfig.MinConns
	config.MaxConnLifetime = p.poolConfig.MaxConnLifetime
	config.MaxConnIdleTime = p.poolConfig.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	p.pools[databaseName] = pool
	return &DatabaseConnection{
		Pool:     pool,
		provider: p,
		dbName:   databaseName,
	}, nil
}

// GetNoRowsSentinel implements pgdbtemplate.ConnectionProvider.GetNoRowsSentinel.
func (*ConnectionProvider) GetNoRowsSentinel() error {
	return pgx.ErrNoRows
}

// Close closes all connection pools managed by this provider.
//
// This should be called when the provider is no longer needed, typically
// in cleanup code or deferred calls. Note that individual DatabaseConnection.Close()
// calls will also close their respective pools, so this is a safety net for
// any remaining pools (e.g., the template database pool).
func (p *ConnectionProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pool := range p.pools {
		pool.Close()
	}
	p.pools = make(map[string]*pgxpool.Pool)
}

// DatabaseConnection implements pgdbtemplate.DatabaseConnection using pgx.
type DatabaseConnection struct {
	Pool     *pgxpool.Pool
	provider *ConnectionProvider
	dbName   string
}

// ExecContext implements pgdbtemplate.DatabaseConnection.ExecContext.
func (c *DatabaseConnection) ExecContext(ctx context.Context, query string, args ...any) (any, error) {
	return c.Pool.Exec(ctx, query, args...)
}

// QueryRowContext implements pgdbtemplate.DatabaseConnection.QueryRowContext.
//
// The returned pgx.Row naturally implements the pgdbtemplate.Row interface.
func (c *DatabaseConnection) QueryRowContext(ctx context.Context, query string, args ...any) pgdbtemplate.Row {
	return c.Pool.QueryRow(ctx, query, args...)
}

// Close implements pgdbtemplate.DatabaseConnection.Close.
//
// This closes and removes the pool for this database from the provider
// if the pool has been created via Connect().
//
// In the pgdbtemplate usage pattern, each test database has a unique name,
// so pools are not shared and can be safely closed when the connection closes.
func (c *DatabaseConnection) Close() error {
	if c.provider == nil {
		// Connection created without provider tracking.
		// Happens if someone creates DatabaseConnection manually.
		c.Pool.Close()
		return nil
	}

	c.provider.mu.Lock()
	defer c.provider.mu.Unlock()

	// Close and remove the pool for this database.
	c.Pool.Close()
	delete(c.provider.pools, c.dbName)
	return nil
}
