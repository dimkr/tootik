/*
Copyright 2023 Dima Krasner

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
	version       = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Parse()

	if version != nil && *version {
		fmt.Println(buildinfo.Version)
		return
	}

	opts := slog.HandlerOptions{Level: slog.Level(cfg.LogLevel)}
	if opts.Level == slog.LevelDebug {
		opts.AddSource = true
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &opts))

	var blockList *fed.BlockList
	if blockListPath != nil && *blockListPath != "" {
		var err error
		blockList, err = fed.NewBlockList(log, *blockListPath)
		if err != nil {
			panic(err)
		}

		defer blockList.Close()
	}

	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	resolver := fed.NewResolver(blockList)

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

	if err := migrations.Run(ctx, log, db); err != nil {
		panic(err)
	}

	if err := data.CollectGarbage(ctx, db); err != nil {
		panic(err)
	}

	nobody, err := user.CreateNobody(ctx, db)
	if err != nil {
		panic(err)
	}

	wg.Add(1)
	go func() {
		if err := fed.ListenAndServe(ctx, db, resolver, nobody, log, *addr, *cert, *key, *plain); err != nil {
			log.Error("HTTPS listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	handler := front.NewHandler(*closed)

	wg.Add(1)
	go func() {
		if err := gemini.ListenAndServe(ctx, log, db, handler, resolver, *gemAddr, *gemCert, *gemKey); err != nil {
			log.Error("Gemini listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := gopher.ListenAndServe(ctx, log, handler, db, resolver, *gopherAddr); err != nil {
			log.Error("Gopher listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := finger.ListenAndServe(ctx, log, db, *fingerAddr); err != nil {
			log.Error("Finger listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := guppy.ListenAndServe(ctx, log, db, handler, resolver, *guppyAddr); err != nil {
			log.Error("Guppy listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		fed.ProcessQueue(ctx, log, db, resolver)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := inbox.ProcessQueue(ctx, log, db, resolver, nobody); err != nil {
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
			if err := outbox.UpdatePollResults(ctx, log, db); err != nil {
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
			if err := outbox.Move(ctx, log, db, resolver, nobody); err != nil {
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
			if err := data.CollectGarbage(ctx, db); err != nil {
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
