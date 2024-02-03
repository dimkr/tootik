/*
Copyright 2023, 2024 Dima Krasner

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

	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/finger"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/gopher"
	"github.com/dimkr/tootik/front/guppy"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/outbox"
	_ "github.com/mattn/go-sqlite3"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	pollResultsUpdateInterval = time.Hour / 2
	garbageCollectionInterval = time.Hour * 12
	followMoveInterval        = time.Hour * 6
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
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
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

	log := slog.New(slog.NewJSONHandler(os.Stderr, &opts))

	var blockList *fed.BlockList
	if *blockListPath != "" {
		var err error
		blockList, err = fed.NewBlockList(log, *blockListPath)
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

	log.Debug("Starting", "version", buildinfo.Version, "cfg", &cfg)

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
	resolver := fed.NewResolver(blockList, *domain, &cfg, &client)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		select {
		case <-sigs:
			log.Info("Received termination signal")
			cancel()
			wg.Done()
		case <-ctx.Done():
			wg.Done()
		}
	}()

	if err := migrations.Run(ctx, log, *domain, db); err != nil {
		panic(err)
	}

	gc := data.GarbageCollector{
		Domain: *domain,
		Config: &cfg,
		DB:     db,
	}
	if err := gc.Run(ctx); err != nil {
		panic(err)
	}

	nobody, err := user.CreateNobody(ctx, *domain, db)
	if err != nil {
		panic(err)
	}

	handler := front.NewHandler(*domain, *closed, &cfg)

	for _, svc := range []struct {
		Name     string
		Listener interface {
			ListenAndServe(context.Context) error
		}
	}{
		{
			"HTTPS",
			&fed.Listener{
				Domain:   *domain,
				LogLevel: slog.Level(*logLevel),
				Config:   &cfg,
				DB:       db,
				Resolver: resolver,
				Actor:    nobody,
				Log:      log,
				Addr:     *addr,
				Cert:     *cert,
				Key:      *key,
				Plain:    *plain,
			},
		},
		{
			"Gemini",
			&gemini.Listener{
				Domain:   *domain,
				Config:   &cfg,
				Log:      log,
				DB:       db,
				Handler:  handler,
				Resolver: resolver,
				Addr:     *gemAddr,
				CertPath: *gemCert,
				KeyPath:  *gemKey,
			},
		},
		{
			"Gopher",
			&gopher.Listener{
				Domain:   *domain,
				Config:   &cfg,
				Log:      log,
				Handler:  handler,
				DB:       db,
				Resolver: resolver,
				Addr:     *gopherAddr,
			},
		},
		{
			"Finger",
			&finger.Listener{
				Domain: *domain,
				Config: &cfg,
				Log:    log,
				DB:     db,
				Addr:   *fingerAddr,
			},
		},
		{
			"Guppy",
			&guppy.Listener{
				Domain:   *domain,
				Config:   &cfg,
				Log:      log,
				DB:       db,
				Handler:  handler,
				Resolver: resolver,
				Addr:     *guppyAddr,
			},
		},
	} {
		l := svc.Listener
		name := svc.Name
		wg.Add(1)
		go func() {
			if err := l.ListenAndServe(ctx); err != nil {
				log.Error("Listener has failed", "name", name, "error", err)
			}
			cancel()
			wg.Done()
		}()
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
				Log:      log,
				DB:       db,
				Resolver: resolver,
				Actor:    nobody,
			},
		},
		{
			"outgoing",
			&fed.Queue{
				Domain:   *domain,
				Config:   &cfg,
				Log:      log,
				DB:       db,
				Resolver: resolver,
			},
		},
	} {
		q := queue.Queue
		name := queue.Name
		wg.Add(1)
		go func() {
			if err := q.Process(ctx); err != nil {
				log.Error("Failed to process queue", "name", name, "error", err)
			}
			cancel()
			wg.Done()
		}()
	}

	for _, job := range []struct {
		Name     string
		Interval time.Duration
		Runner   interface {
			Run(context.Context) error
		}
	}{
		{
			"poller",
			pollResultsUpdateInterval,
			&outbox.Poller{
				Domain: *domain,
				Log:    log,
				DB:     db,
			},
		},
		{
			"mover",
			followMoveInterval,
			&outbox.Mover{
				Domain:   *domain,
				Log:      log,
				DB:       db,
				Resolver: resolver,
				Actor:    nobody,
			},
		},
		{
			"gc",
			garbageCollectionInterval,
			&gc,
		},
	} {
		name := job.Name
		interval := job.Interval
		runner := job.Runner
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()

			t := time.NewTicker(interval)
			defer t.Stop()

			for {
				log.Info("Running periodic job", "name", name)
				if err := runner.Run(ctx); err != nil {
					log.Error("Periodic job has failed", "name", name, "error", err)
					break
				}

				select {
				case <-ctx.Done():
					return

				case <-t.C:
				}
			}
		}()
	}

	<-ctx.Done()
	log.Info("Shutting down")
	wg.Wait()
}
