# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in `pgdbtemplate-pgx`,
please report it responsibly. We take security issues seriously
and will investigate all legitimate reports.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities by creating a
[GitHub Security Advisory](https://github.com/andrei-polukhin/pgdbtemplate-pgx/security/advisories/new).

This ensures:
- The issue is handled confidentially
- Coordinated disclosure with CVE assignment if appropriate
- Proper credit to the reporter

### What to Include

When reporting a vulnerability, please include:
- A clear description of the vulnerability
- Steps to reproduce the issue
- Potential impact and severity
- Any suggested fixes or mitigations

### Response Timeline

- **Initial Response**: Within 48 hours of receiving the report
- **Vulnerability Assessment**: Within 7 days
- **Fix Development**: Within 30 days for critical issues
- **Public Disclosure**: Coordinated with the reporter

## Security Considerations

### Connection String Security

- **Never hardcode credentials** in source code
- Use environment variables or secure credential stores
- Implement proper connection string sanitization
- Avoid logging connection strings containing passwords

### SSL/TLS Configuration

- **Always use SSL/TLS** in production environments
- Configure `sslmode=require` or `sslmode=verify-ca` in connection strings
- Validate server certificates when possible
- Use certificate pinning for high-security environments

### Connection Pooling Security

- Set appropriate connection limits to prevent resource exhaustion
- Configure connection timeouts to prevent hanging connections
- Use `MaxConnLifetime` to rotate connections periodically
- Monitor connection pool metrics for anomalies
- Set minimum connections for consistent performance

### Database Permissions

The connection provider requires minimal database permissions:
- `CONNECT` to target databases
- `CREATE` and `DROP` permissions for test database management
- Read/write permissions for application data operations

Follow the principle of least privilege - grant only necessary permissions.

## Security Scanning

This project implements automated security scanning:

### Static Analysis
- **Gosec**: Scans for common Go security issues
- **Staticcheck**: Advanced static analysis for potential bugs and security issues
- **Govulncheck**: Checks for known vulnerabilities in dependencies

### Dependency Management
- **Dependabot**: Automated weekly dependency updates
- **Go Modules**: Audited dependency graph and checksum verification
- **Vulnerability Monitoring**: Continuous monitoring of security advisories

### CI/CD Security
- **Weekly Security Scans**: Automated security analysis every Monday
- **Dependency Verification**: `go mod verify` ensures integrity
- **Supply Chain Security**: All dependencies are from trusted sources

## Security Best Practices for Users

### Connection Configuration
```go
// Secure connection string example.
connString := "postgres://user:pass@host:5432/db?sslmode=require&sslcert=/path/to/cert&sslkey=/path/to/key&sslrootcert=/path/to/ca"

// Use connection pooling options.
provider := pgdbtemplatepgx.NewConnectionProvider(
    connStringFunc,
    pgdbtemplatepgx.WithMaxConns(10),      // Limit concurrent connections.
    pgdbtemplatepgx.WithMinConns(2),       // Maintain minimum connections.
)
```

### Advanced Pool Configuration
```go
// Custom pool configuration for enhanced security
poolConfig := pgxpool.Config{
    MaxConns:          10,
    MinConns:          2,
    MaxConnLifetime:   time.Hour,
    MaxConnIdleTime:   30 * time.Minute,
    HealthCheckPeriod: 1 * time.Minute,
}

provider := pgdbtemplatepgx.NewConnectionProvider(
    connStringFunc,
    pgdbtemplatepgx.WithPoolConfig(poolConfig),
)
```

### Environment Variables
```bash
# Use environment variables for credentials
export POSTGRES_USER="myuser"
export POSTGRES_PASSWORD="secure_password"
export POSTGRES_SSLMODE="require"
```

### Monitoring
- Monitor connection pool statistics
- Log authentication failures
- Implement connection timeout handling
- Regular security audits of database configurations

## Security Updates

Security updates are released as patch versions and documented in release notes.
Subscribe to GitHub releases to stay informed about security fixes.

### Update Process
1. Security vulnerability identified
2. Fix developed and tested
3. Security advisory published
4. Patch release deployed
5. Users notified via GitHub releases

## Contact

For security-related questions or concerns:
- Security issues: Use [GitHub Security Advisories](https://github.com/andrei-polukhin/pgdbtemplate-pgx/security/advisories/new)
- General questions: Create an issue on GitHub

Thank you for helping keep `pgdbtemplate-pgx` secure!
