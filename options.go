package pgdbtemplatepgx

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionOption configures ConnectionProvider.
type ConnectionOption func(*ConnectionProvider)

// WithPoolConfig sets custom pool configuration.
func WithPoolConfig(config pgxpool.Config) ConnectionOption {
	return func(p *ConnectionProvider) {
		p.poolConfig = config
	}
}

// WithMaxConns sets the maximum number of connections in the pool.
func WithMaxConns(maxConns int32) ConnectionOption {
	return func(p *ConnectionProvider) {
		p.poolConfig.MaxConns = maxConns
	}
}

// WithMinConns sets the minimum number of connections in the pool.
func WithMinConns(minConns int32) ConnectionOption {
	return func(p *ConnectionProvider) {
		p.poolConfig.MinConns = minConns
	}
}
