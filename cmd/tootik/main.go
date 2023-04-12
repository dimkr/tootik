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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/gem"
	"github.com/dimkr/tootik/logger"
	"github.com/dimkr/tootik/user"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	dbPath  = flag.String("db", "db.sqlite3", "database path")
	gemCert = flag.String("gemcert", "gemini-cert.pem", "Gemini TLS certificate")
	gemKey  = flag.String("gemkey", "gemini-key.pem", "Gemini TLS key")
	gemAddr = flag.String("gemaddr", ":8965", "Gemini listening address")
	cert    = flag.String("cert", "cert.pem", "HTTPS TLS certificate")
	key     = flag.String("key", "key.pem", "HTTPS TLS key")
	addr    = flag.String("addr", ":8443", "HTTPS listening address")
)

func main() {
	flag.Parse()

	log := logger.New(nil)

	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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

	if err := data.CollectGarbage(ctx, db); err != nil {
		log.Fatal(err)
	}

	if err := data.Migrate(ctx, db); err != nil {
		log.Fatal(err)
	}

	if err := user.CreateNobody(ctx, db); err != nil {
		log.Fatal(err)
	}

	wg.Add(1)
	go func() {
		if err := fed.ListenAndServe(ctx, db, *addr, *cert, *key); err != nil {
			log.WithError(err).Error("Listener has failed")
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := gem.ListenAndServe(ctx, db, *gemAddr, *gemCert, *gemKey); err != nil {
			log.WithError(err).Error("Listener has failed")
		}
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		fed.DeliverPosts(ctx, db, log)
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
					log.WithError(err).Error("Failed to collect garbage")
				}
			}
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down")
	ticker.Stop()
	wg.Wait()
}
