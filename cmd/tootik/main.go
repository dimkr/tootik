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
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dimkr/slopline"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/text/gmi"
	tplain "github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/gemtext"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/outbox"
	"github.com/dimkr/tootik/sqlite"
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
	cert          = flag.String("cert", "cert.pem", "HTTPS TLS certificate")
	key           = flag.String("key", "key.pem", "HTTPS TLS key")
	addr          = flag.String("addr", ":8443", "HTTPS listening address")
	blockListPath = flag.String("blocklist", "", "Blocklist CSV")
	plain         = flag.Bool("plain", false, "Use HTTP instead of HTTPS")
	cfgPath       = flag.String("cfg", "", "Configuration file")
	dumpCfg       = flag.Bool("dumpcfg", false, "Print default configuration and exit")
	version       = flag.Bool("version", false, "Print version and exit")
)

func shell(
	ctx context.Context,
	handler front.Handler,
	appActor *ap.Actor,
	appActorKeys [2]httpsig.Key,
) error {
	u, err := url.Parse(fmt.Sprintf("gemini://%s/users", *domain))
	if err != nil {
		return err
	}

	prompt := *domain
	var buf bytes.Buffer

	for {
		buf.Reset()

		w := gmi.Wrap(&buf)
		handler.Handle(
			&front.Request{
				Context: ctx,
				URL:     u,
				Log:     slog.Default(),
				User:    appActor,
				Keys:    appActorKeys,
			},
			w,
		)
		w.Flush()

		status, lines, links := gemtext.Parse(buf.String())

		slopline.SetHintsCallback(func(text string) (string, string, string) {
			if text == "" && len(links) > 0 {
				return fmt.Sprintf(" 1-%d", len(links)), "\033[90m", "\033[0m"
			} else if len(links) == 0 {
				return "", "", ""
			}

			if n, err := strconv.Atoi(text); err == nil && n > 0 {
				i := 0
				for _, line := range lines {
					if line.Type != gemtext.Link {
						continue
					}

					i++
					if i == n {
						return " " + line.Text, "\033[90m", "\033[0m"
					}
				}
			}

			return "", "", ""
		})

		if strings.HasPrefix(status, "30 ") {
			rel, _ := url.Parse(status[3 : buf.Len()-2])
			u = u.ResolveReference(rel)
			continue
		}

		if strings.HasPrefix(status, "10 ") {
			prompt = status[3:]
		} else {
			if err := gemtext.Pager(ctx, lines, 80); err != nil {
				return err
			}

			for _, line := range lines {
				if line.Type == gemtext.Heading {
					prompt = line.Text
					break
				}
			}
		}

		line, err := slopline.Line(fmt.Sprintf("\033[35m%s>\033[0m ", prompt))
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if n, err := strconv.Atoi(line); err == nil && n > 0 && n <= len(links) {
			linkID := 1
			for _, line := range lines {
				if line.Type != gemtext.Link {
					continue
				}

				if linkID < n {
					linkID++
					continue
				}

				rel, err := url.Parse(line.URL)
				if err != nil {
					return err
				}

				u = u.ResolveReference(rel)
				break
			}
		} else {
			rel, err := url.Parse(line)
			if err != nil {
				fmt.Printf("Invalid URL or command: %s\n", line)
			}

			u = u.ResolveReference(rel)
		}
	}
}

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
	if !((cmd == "" && flag.NArg() == 0) || (cmd == "shell" && flag.NArg() <= 2) || (cmd == "add-community" && flag.NArg() == 2 && flag.Arg(1) != "") || ((cmd == "set-bio" || cmd == "set-avatar") && flag.NArg() == 3 && flag.Arg(1) != "" && flag.Arg(2) != "")) {
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

	var blockList *fed.BlockList
	if *blockListPath != "" {
		var err error
		blockList, err = fed.NewBlockList(*blockListPath)
		if err != nil {
			panic(err)
		}

		defer blockList.Close()
	}

	db, err := sql.Open(sqlite.DriverName, fmt.Sprintf("%s?%s", *dbPath, cfg.DatabaseOptions))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.MaxDatabaseConnections)

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

	appActor, appActorKeys, err := user.CreateApplicationActor(ctx, *domain, db, &cfg)
	if err != nil {
		panic(err)
	}

	localInbox := &inbox.Inbox{
		Domain: *domain,
		Config: &cfg,
		DB:     db,
	}

	switch cmd {
	case "shell":
		handler, err := front.NewHandler(*domain, &cfg, resolver, db, localInbox)
		if err != nil {
			panic(err)
		}

		if flag.NArg() == 2 {
			var actor ap.Actor
			var rsaPrivKeyDer, ed25519PrivKey []byte
			if err := db.QueryRowContext(
				ctx,
				`select json(actor), rsaprivkey, ed25519privkey from persons where actor->>'$.preferredUsername' = ? and ed25519privkey is not null`,
				flag.Arg(1),
			).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); err != nil {
				panic(err)
			}

			rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
			if err != nil {
				panic(err)
			}

			shell(ctx, handler, &actor, [2]httpsig.Key{
				{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
				{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
			})
		} else if err := shell(ctx, handler, appActor, appActorKeys); err != nil {
			panic(err)
		}

		return

	case "add-community":
		_, _, err := user.CreatePortable(ctx, *domain, db, &cfg, flag.Arg(1), ap.Group, nil)
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
		var ed25519PrivKey []byte
		if err := tx.QueryRowContext(
			ctx,
			`select json(actor), ed25519privkey from persons where ed25519privkey is not null and actor->>'$.preferredUsername' = ?`,
			flag.Arg(1),
		).Scan(&actor, &ed25519PrivKey); err != nil {
			panic(err)
		}

		actor.Summary = tplain.ToHTML(string(summary), nil)
		actor.Updated.Time = time.Now()

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)}); err != nil {
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
		var ed25519PrivKey []byte
		if err := tx.QueryRowContext(
			ctx,
			`select select json(actor), ed25519privkey from persons where ed25519privkey is not null and actor->>'$.preferredUsername' = ?`,
			userName,
		).Scan(&actor, &ed25519PrivKey); err != nil {
			panic(err)
		}

		if _, err := tx.ExecContext(
			ctx,
			"insert into icons(name, buf) values($1, $2) on conflict(name) do update set buf = $2",
			userName,
			danger.String(resized),
		); err != nil {
			panic(err)
		}

		now := time.Now()
		actor.Icon = append(actor.Icon, ap.Attachment{
			URL: fmt.Sprintf("https://%s/icon/%s%s#%d", *domain, userName, icon.FileNameExtension, now.UnixNano()),
		})
		actor.Updated.Time = now

		if err := localInbox.UpdateActorTx(ctx, tx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)}); err != nil {
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
				Domain:       *domain,
				Config:       &cfg,
				DB:           db,
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
				DB:       db,
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
				DB:       db,
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
				Keys:     appActorKeys,
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
				Keys:     appActorKeys,
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
