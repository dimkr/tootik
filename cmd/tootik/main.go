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
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
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

	resolver := fed.NewResolver(blockList, *domain, &cfg)

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

	if err := data.CollectGarbage(ctx, *domain, &cfg, db); err != nil {
		panic(err)
	}

	nobody, err := user.CreateNobody(ctx, *domain, db)
	if err != nil {
		panic(err)
	}

	wg.Add(1)
	go func() {
		if err := fed.ListenAndServe(ctx, *domain, slog.Level(*logLevel), &cfg, db, resolver, nobody, log, *addr, *cert, *key, *plain); err != nil {
			log.Error("HTTPS listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	handler := front.NewHandler(*domain, *closed, &cfg)

	wg.Add(1)
	go func() {
		if err := gemini.ListenAndServe(ctx, *domain, &cfg, log, db, handler, resolver, *gemAddr, *gemCert, *gemKey); err != nil {
			log.Error("Gemini listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := gopher.ListenAndServe(ctx, *domain, &cfg, log, handler, db, resolver, *gopherAddr); err != nil {
			log.Error("Gopher listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := finger.ListenAndServe(ctx, *domain, &cfg, log, db, *fingerAddr); err != nil {
			log.Error("Finger listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := guppy.ListenAndServe(ctx, *domain, &cfg, log, db, handler, resolver, *guppyAddr); err != nil {
			log.Error("Guppy listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		fed.ProcessQueue(ctx, *domain, &cfg, log, db, resolver)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := inbox.ProcessQueue(ctx, *domain, &cfg, log, db, resolver, nobody); err != nil {
			log.Error("Failed to process activities", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		t := time.NewTicker(pollResultsUpdateInterval)
		defer t.Stop()

		for {
			log.Info("Updating poll results")
			if err := outbox.UpdatePollResults(ctx, *domain, log, db); err != nil {
				log.Error("Failed to update poll results", "error", err)
				break
			}

			select {
			case <-ctx.Done():
				return

			case <-t.C:
			}

		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		t := time.NewTicker(followMoveInterval)
		defer t.Stop()

		for {
			if err := outbox.Move(ctx, *domain, log, db, resolver, nobody); err != nil {
				log.Error("Failed to move follows", "error", err)
				break
			}

			select {
			case <-ctx.Done():
				return

			case <-t.C:
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		t := time.NewTicker(garbageCollectionInterval)
		defer t.Stop()

		for {
			log.Info("Collecting garbage")
			if err := data.CollectGarbage(ctx, *domain, &cfg, db); err != nil {
				log.Error("Failed to collect garbage", "error", err)
				break
			}

			select {
			case <-ctx.Done():
				return

			case <-t.C:
			}
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down")
	wg.Wait()
}
