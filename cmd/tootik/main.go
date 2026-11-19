/*
Copyright 2023 - 2026 Dima Krasner

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

package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/gemini"
	tplain "github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/lake"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/outbox"
	"github.com/dimkr/tootik/proof"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	pollResultsUpdateInterval = time.Hour / 2
	garbageCollectionInterval = time.Hour * 12
	followMoveInterval        = time.Hour * 6
	followSyncInterval        = time.Hour * 6
	deleterInterval           = time.Hour * 12
	lakeExportInterval        = time.Hour / 2
	statusUpdateInterval      = time.Hour / 2
)

var (
	domain        = flag.String("domain", "tootik.invalid", "Domain name")
	logLevel      = flag.Int("loglevel", int(slog.LevelInfo), "Logging verbosity")
	dbDSN         = flag.String("db", "postgres://localhost/tootik", "Primary database DSN")
	replicaDSN    = flag.String("replica", "", "Read replica DSN (the primary if empty)")
	lakeDSN       = flag.String("lake", "", "pg_lake database DSN (required to run tootik)")
	lakePublisher = flag.String("lakepublisher", "", "Connection string pg_lake uses to subscribe to the primary (-db if empty)")
	gemCert       = flag.String("gemcert", "", "Gemini TLS certificate")
	gemKey        = flag.String("gemkey", "", "Gemini TLS private key")
	gemAddr       = flag.String("gemaddr", ":1965", "Gemini listening address")
	cert          = flag.String("cert", "", "HTTPS TLS certificate")
	key           = flag.String("key", "", "HTTPS TLS private key")
	addr          = flag.String("addr", ":443", "HTTPS listening address")
	blockListPath = flag.String("blocklist", "", "Blocklist CSV")
	plain         = flag.Bool("plain", false, "Use HTTP instead of HTTPS")
	cfgPath       = flag.String("cfg", "", "Configuration file")
	dumpCfg       = flag.Bool("dumpcfg", false, "Print default configuration and exit")
	version       = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flag]... [arg...]\n", os.Args[0])
		flag.PrintDefaults()

		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flag]...\n\tRun tootik\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flag]... shell [NAME]\n\tRun interactive shell\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flag]... add-community NAME\n\tAdd a community\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flag]... set-bio NAME PATH\n\tSet user's bio\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flag]... set-avatar NAME PATH\n\tSet user's avatar\n", os.Args[0])

		os.Exit(2)
	}
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
	}

	cmd := flag.Arg(0)
	if !((cmd == "" && flag.NArg() == 0 && (*lakeDSN != "" || *dumpCfg)) || (cmd == "shell" && flag.NArg() <= 2) || (cmd == "add-community" && flag.NArg() == 2 && flag.Arg(1) != "") || ((cmd == "set-bio" || cmd == "set-avatar") && flag.NArg() == 3 && flag.Arg(1) != "" && flag.Arg(2) != "")) {
		flag.Usage()
	}

	var cfg cfg.Config

	if *dumpCfg {
		cfg.FillDefaults()
		e := json.NewEncoder(os.Stdout)
		e.SetEscapeHTML(false)
		e.SetIndent("", "\t")
		if err := e.Encode(cfg); err != nil {
			panic(err)
		}
		return
	}

	if *cfgPath != "" {
		f, err := os.Open(*cfgPath)
		if err != nil {
			panic(err)
		}
		if err := json.NewDecoder(f).Decode(&cfg); err != nil {
			f.Close()
			panic(err)
		}
		f.Close()
	}

	cfg.FillDefaults()

	opts := slog.HandlerOptions{Level: slog.Level(*logLevel)}
	if opts.Level == slog.LevelDebug {
		opts.AddSource = true
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &opts)))
	slog.SetLogLoggerLevel(slog.Level(*logLevel))

	var blockList *fed.BlockList
	if *blockListPath != "" {
		var err error
		blockList, err = fed.NewBlockList(*blockListPath)
		if err != nil {
			panic(err)
		}

		defer blockList.Close()
	}

	dbw, err := sql.Open("pgx", *dbDSN)
	if err != nil {
		panic(err)
	}
	defer dbw.Close()

	db := dbw
	if *replicaDSN != "" {
		db, err = sql.Open("pgx", *replicaDSN)
		if err != nil {
			panic(err)
		}
		defer db.Close()
	}

	slog.Debug("Starting", "version", buildinfo.Version, "cfg", &cfg)

	transport := http.Transport{
		MaxIdleConns:    cfg.ResolverMaxIdleConns,
		IdleConnTimeout: cfg.ResolverIdleConnTimeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	client := http.Client{
		Transport: &transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resolver := fed.NewResolver(blockList, *domain, &cfg, &client, dbw)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Go(func() {
		select {
		case <-sigs:
			slog.Info("Received termination signal")
			cancel()
		case <-ctx.Done():
		}
	})

	if err := migrations.Run(ctx, *domain, dbw); err != nil {
		panic(err)
	}

	appActor, appActorKeys, err := user.CreateApplicationActor(ctx, *domain, dbw, &cfg)
	if err != nil {
		panic(err)
	}

	localInbox := &inbox.Inbox{
		Domain: *domain,
		Config: &cfg,
		DBW:    dbw,
	}

	switch cmd {
	case "shell":
		handler, err := front.NewHandler(*domain, &cfg, resolver, db, dbw, localInbox)
		if err != nil {
			panic(err)
		}

		user := "actor"
		if flag.NArg() == 2 {
			user = flag.Arg(1)
		}

		if err := handler.Shell(ctx, user, *domain); err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}

		return

	case "add-community":
		_, _, err := user.CreatePortable(ctx, *domain, dbw, &cfg, flag.Arg(1), ap.Group, nil)
		if err != nil {
			panic(err)
		}
		return

	case "set-bio":
		summary, err := os.ReadFile(flag.Arg(2))
		if err != nil {
			panic(err)
		}

		tx, err := dbw.BeginTx(ctx, nil)
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()

		var actor ap.Actor
		var ed25519Seed, mldsa44Seed []byte
		if err := tx.QueryRowContext(
			ctx,
			`select actor, ed25519seed, mldsa44seed from persons where ed25519seed is not null and actor->>'preferredUsername' = $1`,
			flag.Arg(1),
		).Scan(&actor, &ed25519Seed, &mldsa44Seed); err != nil {
			panic(err)
		}

		actor.Summary = tplain.ToHTML(string(summary), nil)
		actor.Updated.Time = time.Now()

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, proof.SigningSeed(&actor, ed25519Seed, mldsa44Seed)); err != nil {
			panic(err)
		}

		if err := tx.Commit(); err != nil {
			panic(err)
		}

		return

	case "set-avatar":
		avatar, err := os.ReadFile(flag.Arg(2))
		if err != nil {
			panic(err)
		}

		resized, err := icon.Scale(&cfg, avatar)
		if err != nil {
			panic(err)
		}

		tx, err := dbw.BeginTx(ctx, nil)
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()

		userName := flag.Arg(1)

		var actor ap.Actor
		var ed25519Seed, mldsa44Seed []byte
		if err := tx.QueryRowContext(
			ctx,
			`select actor, ed25519seed, mldsa44seed from persons where ed25519seed is not null and actor->>'preferredUsername' = $1`,
			userName,
		).Scan(&actor, &ed25519Seed, &mldsa44Seed); err != nil {
			panic(err)
		}

		if _, err := tx.ExecContext(
			ctx,
			"insert into icons(cid, buf) values($1, $2) on conflict(cid) do update set buf = $2",
			ap.Canonical(actor.ID),
			resized,
		); err != nil {
			panic(err)
		}

		now := time.Now()
		actor.Icon = append(actor.Icon, ap.Attachment{
			URL: fmt.Sprintf("https://%s/icon/%s%s#%d", *domain, userName, icon.FileNameExtension, now.UnixNano()),
		})
		actor.Updated.Time = now

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, proof.SigningSeed(&actor, ed25519Seed, mldsa44Seed)); err != nil {
			panic(err)
		}

		if err := tx.Commit(); err != nil {
			panic(err)
		}

		return
	}

	lakeDB, err := sql.Open("pgx", *lakeDSN)
	if err != nil {
		panic(err)
	}
	defer lakeDB.Close()

	publisher := *lakePublisher
	if publisher == "" {
		publisher = *dbDSN
	}

	if err := lake.Setup(ctx, lakeDB, publisher); err != nil {
		panic(err)
	}

	handler, err := front.NewHandler(*domain, &cfg, resolver, db, dbw, localInbox)
	if err != nil {
		panic(err)
	}

	for _, svc := range []struct {
		Name     string
		Listener interface {
			ListenAndServe(context.Context) error
		}
	}{
		{
			"HTTPS",
			&fed.Listener{
				Domain:       *domain,
				Config:       &cfg,
				DB:           db,
				DBW:          dbw,
				AppActor:     appActor,
				AppActorKeys: appActorKeys,
				Resolver:     resolver,
				Addr:         *addr,
				Cert:         *cert,
				Key:          *key,
				Plain:        *plain,
				BlockList:    blockList,
			},
		},
		{
			"Gemini",
			&gemini.Listener{
				Domain:   *domain,
				Config:   &cfg,
				DBW:      dbw,
				Handler:  handler,
				Addr:     *gemAddr,
				CertPath: *gemCert,
				KeyPath:  *gemKey,
			},
		},
	} {
		wg.Go(func() {
			if err := svc.Listener.ListenAndServe(ctx); err != nil {
				slog.Error("Listener has failed", "listener", svc.Name, "error", err)
			}
			cancel()
		})
	}

	for _, queue := range []struct {
		Name  string
		Queue interface {
			Process(context.Context) error
		}
	}{
		{
			"incoming",
			&inbox.Queue{
				Domain:   *domain,
				Config:   &cfg,
				DBW:      dbw,
				Inbox:    localInbox,
				Resolver: resolver,
				Keys:     appActorKeys,
			},
		},
		{
			"outgoing",
			&fed.Queue{
				Domain:   *domain,
				Config:   &cfg,
				DBW:      dbw,
				Resolver: resolver,
			},
		},
	} {
		wg.Go(func() {
			if err := queue.Queue.Process(ctx); err != nil {
				slog.Error("Failed to process queue", "queue", queue.Name, "error", err)
			}
			cancel()
		})
	}

	for _, job := range []struct {
		Name     string
		Interval time.Duration
		Runner   interface {
			Run(context.Context) error
		}
	}{
		{
			"feed",
			cfg.FeedUpdateInterval,
			&inbox.FeedUpdater{
				Domain: *domain,
				Config: &cfg,
				DBW:    dbw,
			},
		},
		{
			"poller",
			pollResultsUpdateInterval,
			&outbox.Poller{
				Domain: *domain,
				DBW:    dbw,
				Inbox:  localInbox,
			},
		},
		{
			"mover",
			followMoveInterval,
			&outbox.Mover{
				Domain:   *domain,
				DBW:      dbw,
				Inbox:    localInbox,
				Resolver: resolver,
				Keys:     appActorKeys,
			},
		},
		{
			"sync",
			followSyncInterval,
			&fed.Syncer{
				Domain:   *domain,
				Config:   &cfg,
				DBW:      dbw,
				Resolver: resolver,
				Keys:     appActorKeys,
				Inbox:    localInbox,
			},
		},
		{
			"deleter",
			deleterInterval,
			&outbox.Deleter{
				DBW:   dbw,
				Inbox: localInbox,
			},
		},
		{
			"export",
			lakeExportInterval,
			&lake.Exporter{
				Lake: lakeDB,
			},
		},
		{
			"status",
			statusUpdateInterval,
			&lake.StatusUpdater{
				Domain: *domain,
				Lake:   lakeDB,
				DBW:    dbw,
			},
		},
		{
			"gc",
			garbageCollectionInterval,
			&data.GarbageCollector{
				Domain: *domain,
				Config: &cfg,
				DBW:    dbw,
			},
		},
	} {
		wg.Go(func() {
			defer cancel()

			t := time.NewTicker(job.Interval)
			defer t.Stop()

			for {
				slog.Info("Running periodic job", "job", job.Name)
				start := time.Now()
				if err := job.Runner.Run(ctx); err != nil {
					slog.Error("Periodic job has failed", "job", job.Name, "error", err)
					break
				}
				slog.Info("Done running periodic job", "job", job.Name, "duration", time.Since(start).String())

				select {
				case <-ctx.Done():
					return

				case <-t.C:
				}
			}
		})
	}

	<-ctx.Done()
	slog.Info("Shutting down")
	wg.Wait()
}
