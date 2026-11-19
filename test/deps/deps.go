/*
Copyright 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package deps provides the external dependencies of tests.
package deps

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dimkr/tootik/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:18-alpine"
	primaryPort   = "5432/tcp"
	replicaPort   = "5433/tcp"

	// adminDBName is the database initdb always creates. CREATE DATABASE and DROP DATABASE run over a connection to
	// it, because neither can run inside the database it operates on.
	adminDBName = "postgres"

	// templateDBName is migrated once and marked IS_TEMPLATE. Every test database is a CREATE DATABASE ... TEMPLATE
	// copy of it.
	templateDBName = "tootik_template"

	lockRetryInterval = 100 * time.Millisecond
	dropTimeout       = 30 * time.Second

	readyMessage = "tootik: primary and replica are ready"

	// startPostgres runs the primary and a streaming replica of it in one container, then makes every commit wait
	// until the replica applies it. The replica uses a replication slot, so the primary keeps WAL the replica hasn't
	// received yet. The container exits when either server exits.
	startPostgres = `set -e

docker-entrypoint.sh postgres -c fsync=off -c full_page_writes=off -c max_connections=1000 &

until pg_isready -q -h 127.0.0.1 -p 5432 -U postgres; do sleep 0.1; done

gosu postgres pg_basebackup -h /var/run/postgresql -p 5432 -U postgres -D /var/lib/postgresql/replica -R -X stream -C -S replica
chmod 700 /var/lib/postgresql/replica
gosu postgres postgres -D /var/lib/postgresql/replica -p 5433 -c max_connections=1000 -c hot_standby_feedback=on &

until pg_isready -q -h 127.0.0.1 -p 5433 -U postgres; do sleep 0.1; done

psql -v ON_ERROR_STOP=1 -h /var/run/postgresql -p 5432 -U postgres \
	-c "ALTER SYSTEM SET synchronous_standby_names = '*'" \
	-c "ALTER SYSTEM SET synchronous_commit = 'remote_apply'" \
	-c "SELECT pg_reload_conf()"

until [ "$(psql -tA -h /var/run/postgresql -p 5432 -U postgres -c "SELECT count(*) FROM pg_stat_replication WHERE sync_state = 'sync'")" = 1 ]; do sleep 0.1; done

echo "` + readyMessage + `"

wait -n
exit 1
`
)

// T is the subset of [testing.T] used by [NewDatabase].
type T interface {
	Helper()
	Context() context.Context
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
	Cleanup(func())
}

// Database is a test database.
type Database struct {
	// DB is a connection pool to the read replica.
	DB *sql.DB

	// DBW is a connection pool to the primary.
	DBW *sql.DB
}

var seq atomic.Uint64

// shared holds this process's handle on the primary and replica shared by every test binary of a `go test`
// invocation. A failed setup is cached, not retried.
var shared struct {
	sync.Once
	admin       *sql.DB
	primaryAddr string
	replicaAddr string
	err         error
}

func dsn(addr, dbName string) string {
	return fmt.Sprintf("postgres://postgres:postgres@%s/%s?sslmode=disable", addr, dbName)
}

// lockFile takes an exclusive lock on path, shared by every process that locks the same path.
func lockFile(ctx context.Context, path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(lockRetryInterval)
	defer ticker.Stop()

	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, fmt.Errorf("failed to lock %s: %w", path, err)
		}

		select {
		case <-ctx.Done():
			f.Close()
			return nil, fmt.Errorf("failed to lock %s: %w", path, ctx.Err())

		case <-ticker.C:
		}
	}
}

func unlockFile(f *os.File) error {
	return errors.Join(syscall.Flock(int(f.Fd()), syscall.LOCK_UN), f.Close())
}

func containerAddr(ctx context.Context, c *testcontainers.DockerContainer, port string) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}

	mapped, err := c.MappedPort(ctx, port)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(host, mapped.Port()), nil
}

// startContainer starts the container that runs the primary and the replica, or joins the one another test binary of
// this `go test` invocation already started, and returns the addresses of both.
func startContainer(ctx context.Context, sessionID string) (string, string, error) {
	c, err := testcontainers.Run(
		ctx,
		postgresImage,
		testcontainers.WithReuseByName("tootik-pg-"+sessionID),
		testcontainers.WithExposedPorts(primaryPort, replicaPort),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		}),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql": "rw"}),
		testcontainers.WithEntrypoint("bash", "-c", startPostgres),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog(readyMessage),
				wait.ForListeningPort(primaryPort),
				wait.ForListeningPort(replicaPort),
			),
		),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to start Postgres: %w", err)
	}

	primaryAddr, err := containerAddr(ctx, c, primaryPort)
	if err != nil {
		return "", "", fmt.Errorf("failed to get the primary's address: %w", err)
	}

	replicaAddr, err := containerAddr(ctx, c, replicaPort)
	if err != nil {
		return "", "", fmt.Errorf("failed to get the replica's address: %w", err)
	}

	return primaryAddr, replicaAddr, nil
}

// ensureMigrated migrates the template database and marks it IS_TEMPLATE, unless another test binary already did.
// IS_TEMPLATE is set only after all migrations succeed, so a database left behind by an interrupted migration is
// dropped and migrated again.
func ensureMigrated(ctx context.Context, admin *sql.DB, primaryAddr string) error {
	var isTemplate bool
	if err := admin.QueryRowContext(ctx, `select datistemplate from pg_database where datname = $1`, templateDBName).Scan(&isTemplate); err == nil && isTemplate {
		return nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check if %s is a template: %w", templateDBName, err)
	}

	if _, err := admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+templateDBName+` WITH (FORCE)`); err != nil {
		return fmt.Errorf("failed to drop %s: %w", templateDBName, err)
	}

	if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+templateDBName); err != nil {
		return fmt.Errorf("failed to create %s: %w", templateDBName, err)
	}

	db, err := sql.Open("pgx", dsn(primaryAddr, templateDBName))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", templateDBName, err)
	}

	migrateErr := migrations.Run(ctx, "", db)

	// Postgres refuses to mark a database as a template while other sessions are connected to it
	if err := errors.Join(migrateErr, db.Close()); err != nil {
		return fmt.Errorf("failed to migrate %s: %w", templateDBName, err)
	}

	if _, err := admin.ExecContext(ctx, `ALTER DATABASE `+templateDBName+` WITH IS_TEMPLATE TRUE`); err != nil {
		return fmt.Errorf("failed to mark %s as a template: %w", templateDBName, err)
	}

	return nil
}

// setup starts or joins the container and migrates the template database under a lock shared by every test binary
// of this `go test` invocation, so they do this once between them.
func setup(ctx context.Context) error {
	sessionID := testcontainers.SessionID()

	lock, err := lockFile(ctx, filepath.Join(os.TempDir(), "tootik-pg-"+sessionID+".lock"))
	if err != nil {
		return err
	}

	primaryAddr, replicaAddr, err := startContainer(ctx, sessionID)
	if err != nil {
		return errors.Join(err, unlockFile(lock))
	}

	admin, err := sql.Open("pgx", dsn(primaryAddr, adminDBName))
	if err != nil {
		return errors.Join(fmt.Errorf("failed to connect to %s: %w", adminDBName, err), unlockFile(lock))
	}

	if err := ensureMigrated(ctx, admin, primaryAddr); err != nil {
		return errors.Join(err, admin.Close(), unlockFile(lock))
	}

	if err := unlockFile(lock); err != nil {
		return errors.Join(err, admin.Close())
	}

	shared.admin = admin
	shared.primaryAddr = primaryAddr
	shared.replicaAddr = replicaAddr
	return nil
}

// NewDatabase creates a test database by copying a migrated template, on a primary and read replica shared by every
// test binary of this `go test` invocation. Every commit on the primary waits until the replica applies it. The
// database is dropped when the test ends.
func NewDatabase(t T) *Database {
	t.Helper()

	shared.Do(func() {
		shared.err = setup(t.Context())
	})
	if shared.err != nil {
		t.Fatalf("Failed to start Postgres: %v", shared.err)
	}

	dbName := fmt.Sprintf("test_%d_%d", os.Getpid(), seq.Add(1))
	if _, err := shared.admin.ExecContext(t.Context(), `CREATE DATABASE `+dbName+` TEMPLATE `+templateDBName); err != nil {
		t.Fatalf("Failed to create %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), dropTimeout)
		defer cancel()

		if _, err := shared.admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+dbName+` WITH (FORCE)`); err != nil {
			t.Errorf("Failed to drop %s: %v", dbName, err)
		}
	})

	dbw, err := sql.Open("pgx", dsn(shared.primaryAddr, dbName))
	if err != nil {
		t.Fatalf("Failed to connect to %s on the primary: %v", dbName, err)
	}

	t.Cleanup(func() {
		if err := dbw.Close(); err != nil {
			t.Errorf("Failed to close %s on the primary: %v", dbName, err)
		}
	})

	db, err := sql.Open("pgx", dsn(shared.replicaAddr, dbName))
	if err != nil {
		t.Fatalf("Failed to connect to %s on the replica: %v", dbName, err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close %s on the replica: %v", dbName, err)
		}
	})

	return &Database{DB: db, DBW: dbw}
}
