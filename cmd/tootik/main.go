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
	"log/slog"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front/finger"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/gopher"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/user"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	dbPath     = flag.String("db", "db.sqlite3", "database path")
	gemCert    = flag.String("gemcert", "gemini-cert.pem", "Gemini TLS certificate")
	gemKey     = flag.String("gemkey", "gemini-key.pem", "Gemini TLS key")
	gemAddr    = flag.String("gemaddr", ":8965", "Gemini listening address")
	gopherAddr = flag.String("gopheraddr", ":8070", "Gopher listening address")
	fingerAddr = flag.String("fingeraddr", ":8079", "Finger listening address")
	cert       = flag.String("cert", "cert.pem", "HTTPS TLS certificate")
	key        = flag.String("key", "key.pem", "HTTPS TLS key")
	addr       = flag.String("addr", ":8443", "HTTPS listening address")
)

func main() {
	flag.Parse()

	logLevel := slog.Level(cfg.LogLevel)
	var opts slog.HandlerOptions
	if logLevel == slog.LevelDebug {
		opts.AddSource = true
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &opts))

	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	resolver := fed.NewResolver()

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

	if err := user.CreateNobody(ctx, db); err != nil {
		panic(err)
	}

	wg.Add(1)
	go func() {
		if err := fed.ListenAndServe(ctx, db, resolver, log, *addr, *cert, *key); err != nil {
			log.Error("HTTPS listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := gemini.ListenAndServe(ctx, log, db, resolver, *gemAddr, *gemCert, *gemKey); err != nil {
			log.Error("Gemini listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := gopher.ListenAndServe(ctx, log, db, resolver, *gopherAddr); err != nil {
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
		fed.DeliverPosts(ctx, log, db, resolver)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := fed.ProcessActivities(ctx, log, db, resolver); err != nil {
			log.Error("Failed to process activities", "error", err)
		}
		cancel()
		wg.Done()
	}()

	ticker := time.NewTicker(time.Hour * 12)

	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return

			case <-ticker.C:
				log.Info("Collecting garbage")
				if err := data.CollectGarbage(ctx, db); err != nil {
					log.Error("Failed to collect garbage", "error", err)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down")
	ticker.Stop()
	wg.Wait()
}
