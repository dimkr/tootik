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
	"encoding/csv"
	"flag"
	"io"
	"log/slog"
	"os"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/finger"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/gopher"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/user"
	_ "github.com/mattn/go-sqlite3"
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
	blockList  = flag.String("blocklist", "", "Blocklist CSV")
)

func main() {
	flag.Parse()

	opts := slog.HandlerOptions{Level: slog.Level(cfg.LogLevel)}
	if opts.Level == slog.LevelDebug {
		opts.AddSource = true
	}

	blockedDomains := make(map[string]struct{})
	if blockList != nil && *blockList != "" {
		f, err := os.Open(*blockList)
		if err != nil {
			panic(err)
		}

		c := csv.NewReader(f)
		first := true
		for {
			r, err := c.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}

			if first {
				first = false
				continue
			}

			blockedDomains[r[0]] = struct{}{}
		}

		f.Close()
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &opts))

	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	resolver := fed.NewResolver(blockedDomains)

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
		if err := fed.ListenAndServe(ctx, db, resolver, nobody, log, *addr, *cert, *key); err != nil {
			log.Error("HTTPS listener has failed", "error", err)
		}
		cancel()
		wg.Done()
	}()

	handler := front.NewHandler()

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
