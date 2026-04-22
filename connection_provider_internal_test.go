package pgdbtemplatepgx

import (
	"context"
	"os"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/andrei-polukhin/pgdbtemplate"
)

// TestConnectDoubleCheckPathWithoutHook exercises the write-lock double-check
// path in Connect without any production test hooks.
func TestConnectDoubleCheckPathWithoutHook(t *testing.T) {
	t.Parallel()
	c := qt.New(t)
	ctx := context.Background()

	baseConnString := os.Getenv("POSTGRES_CONNECTION_STRING")
	c.Assert(baseConnString == "", qt.IsFalse)

	provider := NewConnectionProvider(func(dbName string) string {
		return pgdbtemplate.ReplaceDatabaseInConnectionString(baseConnString, dbName)
	})
	defer provider.Close()

	const (
		rounds     = 100
		goroutines = 16
	)

	for round := 0; round < rounds; round++ {
		start := make(chan struct{})
		results := make(chan error, goroutines)
		conns := make(chan pgdbtemplate.DatabaseConnection, goroutines)

		// Hold write lock so all goroutines queue at the top of Connect first,
		// then release together to maximize overlapping cold-path execution.
		provider.mu.Lock()
		for i := 0; i < goroutines; i++ {
			go func() {
				<-start
				conn, err := provider.Connect(ctx, "postgres")
				results <- err
				conns <- conn
			}()
		}
		close(start)
		provider.mu.Unlock()

		allConns := make([]pgdbtemplate.DatabaseConnection, 0, goroutines)
		for i := 0; i < goroutines; i++ {
			err := <-results
			c.Assert(err, qt.IsNil)
			allConns = append(allConns, <-conns)
		}

		var wg sync.WaitGroup
		for _, conn := range allConns {
			wg.Add(1)
			go func(dc pgdbtemplate.DatabaseConnection) {
				defer wg.Done()
				c.Assert(dc.Close(), qt.IsNil)
			}(conn)
		}
		wg.Wait()
	}
}
