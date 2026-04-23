package pgdbtemplatepgx

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// poolEntry holds a connection pool and its active reference count.
type poolEntry struct {
	pool *pgxpool.Pool
	refs atomic.Int32
}

// ConnectionProvider implements pgdbtemplate.ConnectionProvider
// using pgx driver with connection pooling.
type ConnectionProvider struct {
	connectionStringFunc func(string) string
	poolConfig           pgxpool.Config

	mu    sync.RWMutex
	pools map[string]*poolEntry
}

// NewConnectionProvider creates a new pgx-based connection provider.
func NewConnectionProvider(connectionStringFunc func(string) string, opts ...ConnectionOption) *ConnectionProvider {
	provider := &ConnectionProvider{
		connectionStringFunc: connectionStringFunc,
		pools:                make(map[string]*poolEntry),
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
	if entry, exists := p.pools[databaseName]; exists {
		entry.refs.Add(1)
		p.mu.RUnlock()
		return &DatabaseConnection{
			Pool:     entry.pool,
			provider: p,
			dbName:   databaseName,
		}, nil
	}
	p.mu.RUnlock()

	// Create new pool.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if entry, exists := p.pools[databaseName]; exists {
		entry.refs.Add(1)
		return &DatabaseConnection{Pool: entry.pool, provider: p, dbName: databaseName}, nil
	}

	// Parse connection string first.
	connString := p.connectionStringFunc(databaseName)
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	p.applyPoolConfig(config)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	entry := &poolEntry{pool: pool}
	entry.refs.Add(1)
	p.pools[databaseName] = entry
	return &DatabaseConnection{
		Pool:     pool,
		provider: p,
		dbName:   databaseName,
	}, nil
}

// applyPoolConfig merges user-provided pool options into a parsed config,
// preserving pgx defaults for fields where zero has special meaning.
//
// ConnConfig is intentionally not copied from p.poolConfig to preserve
// Connect(databaseName) behavior that derives the target database from the
// parsed connection string for each call.
func (p *ConnectionProvider) applyPoolConfig(config *pgxpool.Config) {
	// HealthCheckPeriod must be checked (pgx validates > 0).
	if p.poolConfig.HealthCheckPeriod != 0 {
		config.HealthCheckPeriod = p.poolConfig.HealthCheckPeriod
	}
	// MaxConns must be checked (pgx validates >= 1).
	if p.poolConfig.MaxConns != 0 {
		config.MaxConns = p.poolConfig.MaxConns
	}

	// MinConns: 0 is a valid value (no minimum), assign unconditionally.
	config.MinConns = p.poolConfig.MinConns
	if p.poolConfig.MaxConnLifetime != 0 {
		config.MaxConnLifetime = p.poolConfig.MaxConnLifetime
	}
	if p.poolConfig.MaxConnIdleTime != 0 {
		config.MaxConnIdleTime = p.poolConfig.MaxConnIdleTime
	}
	if p.poolConfig.MaxConnLifetimeJitter != 0 {
		config.MaxConnLifetimeJitter = p.poolConfig.MaxConnLifetimeJitter
	}
	if p.poolConfig.BeforeConnect != nil {
		config.BeforeConnect = p.poolConfig.BeforeConnect
	}
	if p.poolConfig.AfterConnect != nil {
		config.AfterConnect = p.poolConfig.AfterConnect
	}
	if p.poolConfig.BeforeAcquire != nil {
		config.BeforeAcquire = p.poolConfig.BeforeAcquire
	}
	if p.poolConfig.AfterRelease != nil {
		config.AfterRelease = p.poolConfig.AfterRelease
	}
	if p.poolConfig.BeforeClose != nil {
		config.BeforeClose = p.poolConfig.BeforeClose
	}
}

// GetNoRowsSentinel implements pgdbtemplate.ConnectionProvider.GetNoRowsSentinel.
func (*ConnectionProvider) GetNoRowsSentinel() error {
	return pgx.ErrNoRows
}

// Close closes all connection pools managed by this provider.
//
// This should be called when the provider is no longer needed, typically
// at the end of a test suite. It forcefully closes all pools regardless
// of any outstanding references.
func (p *ConnectionProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, entry := range p.pools {
		entry.pool.Close()
	}
	p.pools = make(map[string]*poolEntry)
}

// DatabaseConnection implements pgdbtemplate.DatabaseConnection using pgx.
type DatabaseConnection struct {
	Pool      *pgxpool.Pool
	provider  *ConnectionProvider
	dbName    string
	closeOnce sync.Once
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
// It decrements the reference count of the underlying pool. The pool is
// closed and removed from the provider only when the last reference is
// released. Calling Close more than once is safe and has no effect after
// the first call.
func (c *DatabaseConnection) Close() error {
	if c.provider == nil {
		// Connection created without provider tracking.
		// Happens if someone creates DatabaseConnection manually.
		c.closeOnce.Do(c.Pool.Close)
		return nil
	}

	c.closeOnce.Do(func() {
		c.provider.mu.Lock()
		defer c.provider.mu.Unlock()

		entry, exists := c.provider.pools[c.dbName]
		if !exists {
			// Pool was already removed, e.g. by provider.Close().
			return
		}
		if entry.refs.Add(-1) == 0 {
			entry.pool.Close()
			delete(c.provider.pools, c.dbName)
		}
	})
	return nil
}
