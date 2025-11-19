/*
Copyright 2023 - 2025 Dima Krasner

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
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"

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
	"github.com/dimkr/tootik/front/finger"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/gopher"
	"github.com/dimkr/tootik/front/guppy"
	tplain "github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/outbox"
	_ "github.com/mattn/go-sqlite3"
)

const (
	pollResultsUpdateInterval = time.Hour / 2
	garbageCollectionInterval = time.Hour * 12
	followMoveInterval        = time.Hour * 6
	followSyncInterval        = time.Hour * 6
	deleterInterval           = time.Hour * 12
)

var (
	domain        = flag.String("domain", "localhost.localdomain:8443", "Domain name")
	logLevel      = flag.Int("loglevel", int(slog.LevelInfo), "Logging verbosity")
	dbPath        = flag.String("db", "db.sqlite3", "database path")
	gemCert       = flag.String("gemcert", "gemini-cert.pem", "Gemini TLS certificate")
	gemKey        = flag.String("gemkey", "gemini-key.pem", "Gemini TLS key")
	gemAddr       = flag.String("gemaddr", ":8965", "Gemini listening address")
	gopherAddr    = flag.String("gopheraddr", ":8070", "Gopher listening address")
	fingerAddr    = flag.String("fingeraddr", ":8079", "Finger listening address")
	guppyAddr     = flag.String("guppyaddr", ":6775", "Guppy listening address")
	cert          = flag.String("cert", "cert.pem", "HTTPS TLS certificate")
	key           = flag.String("key", "key.pem", "HTTPS TLS key")
	addr          = flag.String("addr", ":8443", "HTTPS listening address")
	blockListPath = flag.String("blocklist", "", "Blocklist CSV")
	closed        = flag.Bool("closed", false, "Disable new user registration")
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
	if !((cmd == "" && flag.NArg() == 0) || (cmd == "add-community" && flag.NArg() == 2 && flag.Arg(1) != "") || ((cmd == "set-bio" || cmd == "set-avatar") && flag.NArg() == 3 && flag.Arg(1) != "" && flag.Arg(2) != "")) {
		flag.Usage()
	}

	uuid.EnableRandPool()

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

	if *closed {
		slog.Warn("-closed is deprecated, set RequireInvite and MaxInvitesPerUser instead")
		cfg.RequireInvitation = true
		cfg.MaxInvitationsPerUser = new(int)
	}

	var blockList *fed.BlockList
	if *blockListPath != "" {
		var err error
		blockList, err = fed.NewBlockList(*blockListPath)
		if err != nil {
			panic(err)
		}

		defer blockList.Close()
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?%s", *dbPath, cfg.DatabaseOptions))
	if err != nil {
		panic(err)
	}
	defer db.Close()

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
	resolver := fed.NewResolver(blockList, *domain, &cfg, &client, db)

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

	if err := migrations.Run(ctx, *domain, db); err != nil {
		panic(err)
	}

	_, nobodyKeys, err := user.CreateNobody(ctx, *domain, db, &cfg)
	if err != nil {
		panic(err)
	}

	localInbox := &inbox.Inbox{
		Domain: *domain,
		Config: &cfg,
		DB:     db,
	}

	switch cmd {
	case "add-community":
		_, _, err := user.Create(ctx, *domain, db, &cfg, flag.Arg(1), ap.Group, nil)
		if err != nil {
			panic(err)
		}
		return

	case "set-bio":
		summary, err := os.ReadFile(flag.Arg(2))
		if err != nil {
			panic(err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()

		var actor ap.Actor
		var ed25519PrivKeyMultibase string
		if err := tx.QueryRowContext(
			ctx,
			`select json(actor), ed25519privkey from persons where ed25519privkey is not null and actor->>'$.preferredUsername' = ?`,
			flag.Arg(1),
		).Scan(&actor, &ed25519PrivKeyMultibase); err != nil {
			panic(err)
		}

		ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
		if err != nil {
			panic(err)
		}

		actor.Summary = tplain.ToHTML(string(summary), nil)
		actor.Updated.Time = time.Now()

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}); err != nil {
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

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()

		userName := flag.Arg(1)

		var actor ap.Actor
		var ed25519PrivKeyMultibase string
		if err := tx.QueryRowContext(
			ctx,
			`select select json(actor), ed25519privkey from persons where ed25519privkey is not null and actor->>'$.preferredUsername' = ?`,
			userName,
		).Scan(&actor, &ed25519PrivKeyMultibase); err != nil {
			panic(err)
		}

		ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
		if err != nil {
			panic(err)
		}

		if _, err := tx.ExecContext(
			ctx,
			"insert into icons(name, buf) values($1, $2) on conflict(name) do update set buf = $2",
			userName,
			string(resized),
		); err != nil {
			panic(err)
		}

		now := time.Now()
		actor.Icon = append(actor.Icon, ap.Attachment{
			URL: fmt.Sprintf("https://%s/icon/%s%s#%d", *domain, userName, icon.FileNameExtension, now.UnixNano()),
		})
		actor.Updated.Time = now

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}); err != nil {
			panic(err)
		}

		if err := tx.Commit(); err != nil {
			panic(err)
		}

		return
	}

	handler, err := front.NewHandler(*domain, &cfg, resolver, db, localInbox)
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
				Domain:    *domain,
				Closed:    *closed,
				Config:    &cfg,
				DB:        db,
				ActorKeys: nobodyKeys,
				Resolver:  resolver,
				Addr:      *addr,
				Cert:      *cert,
				Key:       *key,
				Plain:     *plain,
				BlockList: blockList,
			},
		},
		{
			"Gemini",
			&gemini.Listener{
				Domain:   *domain,
				Config:   &cfg,
				DB:       db,
				Handler:  handler,
				Addr:     *gemAddr,
				CertPath: *gemCert,
				KeyPath:  *gemKey,
			},
		},
		{
			"Gopher",
			&gopher.Listener{
				Domain:  *domain,
				Config:  &cfg,
				Handler: handler,
				Addr:    *gopherAddr,
			},
		},
		{
			"Finger",
			&finger.Listener{
				Domain: *domain,
				Config: &cfg,
				DB:     db,
				Addr:   *fingerAddr,
			},
		},
		{
			"Guppy",
			&guppy.Listener{
				Domain:  *domain,
				Config:  &cfg,
				Handler: handler,
				Addr:    *guppyAddr,
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
				Config:   &cfg,
				DB:       db,
				Inbox:    localInbox,
				Resolver: resolver,
				Keys:     nobodyKeys,
			},
		},
		{
			"outgoing",
			&fed.Queue{
				Domain:   *domain,
				Config:   &cfg,
				DB:       db,
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
				DB:     db,
			},
		},
		{
			"poller",
			pollResultsUpdateInterval,
			&outbox.Poller{
				Domain: *domain,
				DB:     db,
				Inbox:  localInbox,
			},
		},
		{
			"mover",
			followMoveInterval,
			&outbox.Mover{
				Domain:   *domain,
				DB:       db,
				Inbox:    localInbox,
				Resolver: resolver,
				Keys:     nobodyKeys,
			},
		},
		{
			"sync",
			followSyncInterval,
			&fed.Syncer{
				Domain:   *domain,
				Config:   &cfg,
				DB:       db,
				Resolver: resolver,
				Keys:     nobodyKeys,
				Inbox:    localInbox,
			},
		},
		{
			"deleter",
			deleterInterval,
			&outbox.Deleter{
				DB:    db,
				Inbox: localInbox,
			},
		},
		{
			"gc",
			garbageCollectionInterval,
			&data.GarbageCollector{
				Domain: *domain,
				Config: &cfg,
				DB:     db,
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
