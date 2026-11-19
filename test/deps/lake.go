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

package deps

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/dimkr/tootik/lake"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// lakeImage is built by build-pg-lake.sh, and its tag must match PG_LAKE_VERSION there.
	lakeImage = "tootik-pg-lake:v3.6.2"
	lakePort  = "5432/tcp"

	rustfsImage     = "rustfs/rustfs:1.0.6"
	rustfsPort      = "9000/tcp"
	rustfsAlias     = "rustfs"
	rustfsAccessKey = "tootik"
	rustfsSecretKey = "tootiktootik"
	bucket          = "tootik"

	lakeReadyMessage = "tootik: pg_lake is ready"

	syncRetryInterval = 100 * time.Millisecond

	// pgduckInit is run by pgduck_server on startup, to let it access RustFS.
	pgduckInit = `CREATE SECRET rustfs (
	TYPE s3,
	SCOPE 's3://` + bucket + `',
	USE_SSL false,
	KEY_ID '` + rustfsAccessKey + `',
	SECRET '` + rustfsSecretKey + `',
	URL_STYLE 'path',
	ENDPOINT '` + rustfsAlias + `:9000',
	REGION 'us-east-1'
);
`

	// startLake runs pgduck_server and a pg_lake server that uses it in one container. The container exits when
	// either process exits.
	startLake = `set -e

data=$PGBASEDIR/pgsql-$PG_MAJOR/data
socket=$PGBASEDIR/pgduck_socket_dir

mkdir -p $socket $PGBASEDIR/cache
chmod 700 $socket

printf 'local all all trust\nhost all all 0.0.0.0/0 trust\nhost all all ::/0 trust\n' > $data/pg_hba.conf

pgduck_server --cache_dir $PGBASEDIR/cache --unix_socket_directory $socket --port 5332 --init_file_path /init-pgduck-server.sql &

until pg_isready -q -h $socket -p 5332; do sleep 0.1; done

postgres -D $data \
	-c listen_addresses='*' \
	-c port=5432 \
	-c shared_preload_libraries=pg_extension_base \
	-c "pg_lake_engine.host=host=$socket port=5332" \
	-c fsync=off \
	-c full_page_writes=off \
	-c max_connections=1000 \
	-c max_worker_processes=256 \
	-c max_logical_replication_workers=128 \
	-c max_sync_workers_per_subscription=4 \
	-c max_active_replication_origins=128 \
	-c max_replication_slots=128 &

until pg_isready -q -h 127.0.0.1 -p 5432 -U postgres; do sleep 0.1; done

echo "` + lakeReadyMessage + `"

wait -n
exit 1
`
)

// lakeShared holds this process's handle on the pg_lake server shared by every test binary of a `go test`
// invocation. A failed setup is cached, not retried.
var lakeShared struct {
	sync.Once
	admin *sql.DB
	addr  string
	err   error
}

// checkLakeImage fails with instructions if the pg_lake image was not built.
func checkLakeImage(ctx context.Context) error {
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker: %w", err)
	}
	defer cli.Close()

	if _, err := cli.ImageInspect(ctx, lakeImage); cerrdefs.IsNotFound(err) {
		return fmt.Errorf("run test/deps/build-pg-lake.sh to build %s", lakeImage)
	} else if err != nil {
		return fmt.Errorf("failed to inspect %s: %w", lakeImage, err)
	}

	return nil
}

// startRustFS starts RustFS and creates the bucket, or joins the RustFS container another test binary of this
// `go test` invocation already started.
func startRustFS(ctx context.Context, sessionID string) error {
	c, err := testcontainers.Run(
		ctx,
		rustfsImage,
		testcontainers.WithReuseByName("tootik-rustfs-"+sessionID),
		network.WithNetworkName([]string{rustfsAlias}, networkPrefix+sessionID),
		testcontainers.WithExposedPorts(rustfsPort),
		testcontainers.WithEnv(map[string]string{
			"RUSTFS_ACCESS_KEY":     rustfsAccessKey,
			"RUSTFS_SECRET_KEY":     rustfsSecretKey,
			"RUSTFS_CONSOLE_ENABLE": "false",
		}),
		testcontainers.WithTmpfs(map[string]string{"/data": "rw,uid=10001,gid=10001"}),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/health/ready").WithPort(rustfsPort)),
	)
	if err != nil {
		return fmt.Errorf("failed to start RustFS: %w", err)
	}

	for _, cmd := range [][]string{
		{"rc", "alias", "set", "local", "http://127.0.0.1:9000", rustfsAccessKey, rustfsSecretKey},
		{"rc", "bucket", "create", "--ignore-existing", "local/" + bucket},
	} {
		code, out, err := c.Exec(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to run %s: %w", strings.Join(cmd, " "), err)
		}

		if code != 0 {
			output, _ := io.ReadAll(out)
			return fmt.Errorf("%s exited with %d: %s", strings.Join(cmd, " "), code, output)
		}
	}

	return nil
}

// startLakeContainer starts the pg_lake container, or joins the one another test binary of this `go test` invocation
// already started, and returns its address.
func startLakeContainer(ctx context.Context, sessionID string) (string, error) {
	c, err := testcontainers.Run(
		ctx,
		lakeImage,
		testcontainers.WithReuseByName("tootik-lake-"+sessionID),
		network.WithNetworkName([]string{"lake"}, networkPrefix+sessionID),
		testcontainers.WithExposedPorts(lakePort),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			Reader:            strings.NewReader(pgduckInit),
			ContainerFilePath: "/init-pgduck-server.sql",
			FileMode:          0o644,
		}),
		testcontainers.WithEntrypoint("bash", "-c", startLake),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog(lakeReadyMessage),
				wait.ForListeningPort(lakePort),
			),
		),
	)
	if err != nil {
		return "", fmt.Errorf("failed to start pg_lake: %w", err)
	}

	return containerAddr(ctx, c, lakePort)
}

// setupLake starts or joins RustFS and pg_lake under a lock shared by every test binary of this `go test` invocation.
func setupLake(ctx context.Context) error {
	if err := checkLakeImage(ctx); err != nil {
		return err
	}

	sessionID := testcontainers.SessionID()

	lock, err := lockFile(ctx, filepath.Join(os.TempDir(), "tootik-lake-"+sessionID+".lock"))
	if err != nil {
		return err
	}

	if err := startRustFS(ctx, sessionID); err != nil {
		return errors.Join(err, unlockFile(lock))
	}

	addr, err := startLakeContainer(ctx, sessionID)
	if err != nil {
		return errors.Join(err, unlockFile(lock))
	}

	admin, err := sql.Open("pgx", dsn(addr, adminDBName))
	if err != nil {
		return errors.Join(fmt.Errorf("failed to connect to %s on pg_lake: %w", adminDBName, err), unlockFile(lock))
	}

	// pg_lake creates roles shared by all databases, so concurrent CREATE EXTENSION in two new databases fails unless
	// the roles already exist
	if _, err := admin.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pg_lake CASCADE`); err != nil {
		return errors.Join(fmt.Errorf("failed to create pg_lake in %s: %w", adminDBName, err), admin.Close(), unlockFile(lock))
	}

	if err := unlockFile(lock); err != nil {
		return errors.Join(err, admin.Close())
	}

	lakeShared.admin = admin
	lakeShared.addr = addr
	return nil
}

// waitForSync waits until the initial copy of every subscribed table is complete.
func waitForSync(ctx context.Context, db *sql.DB) error {
	ticker := time.NewTicker(syncRetryInterval)
	defer ticker.Stop()

	for {
		var syncing int64
		if err := db.QueryRowContext(ctx, `select count(*) from pg_subscription_rel join pg_subscription on pg_subscription.oid = pg_subscription_rel.srsubid where pg_subscription.subname = current_database() and pg_subscription_rel.srsubstate != 'r'`).Scan(&syncing); err != nil {
			return err
		}

		if syncing == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
		}
	}
}

// NewLake creates a pg_lake database that subscribes to db, with Iceberg tables stored in RustFS. The pg_lake server
// and RustFS are shared by every test binary of this `go test` invocation. The database and its subscription are
// dropped when the test ends.
func NewLake(t T, db *Database) *sql.DB {
	t.Helper()

	lakeShared.Do(func() {
		lakeShared.err = setupLake(t.Context())
	})
	if lakeShared.err != nil {
		t.Fatalf("Failed to start pg_lake: %v", lakeShared.err)
	}

	if _, err := lakeShared.admin.ExecContext(t.Context(), `CREATE DATABASE `+db.name); err != nil {
		t.Fatalf("Failed to create %s on pg_lake: %v", db.name, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), dropTimeout)
		defer cancel()

		if _, err := lakeShared.admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+db.name+` WITH (FORCE)`); err != nil {
			t.Errorf("Failed to drop %s on pg_lake: %v", db.name, err)
		}
	})

	if _, err := lakeShared.admin.ExecContext(t.Context(), `ALTER DATABASE `+db.name+` SET pg_lake_iceberg.default_location_prefix = 's3://`+bucket+`/`+db.name+`'`); err != nil {
		t.Fatalf("Failed to set Iceberg location of %s: %v", db.name, err)
	}

	lakeDB, err := sql.Open("pgx", dsn(lakeShared.addr, db.name))
	if err != nil {
		t.Fatalf("Failed to connect to %s on pg_lake: %v", db.name, err)
	}

	t.Cleanup(func() {
		if err := lakeDB.Close(); err != nil {
			t.Errorf("Failed to close %s on pg_lake: %v", db.name, err)
		}
	})

	if err := lake.Setup(t.Context(), lakeDB, fmt.Sprintf("host=%s port=5432 dbname=%s user=postgres password=postgres", postgresAlias, db.name)); err != nil {
		t.Fatalf("Failed to set up %s on pg_lake: %v", db.name, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), dropTimeout)
		defer cancel()

		if _, err := lakeDB.ExecContext(ctx, `DROP SUBSCRIPTION IF EXISTS `+db.name); err != nil {
			t.Errorf("Failed to drop subscription %s: %v", db.name, err)
		}
	})

	if err := waitForSync(t.Context(), lakeDB); err != nil {
		t.Fatalf("Failed to wait for initial synchronization of %s: %v", db.name, err)
	}

	return lakeDB
}
