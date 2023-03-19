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
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)

	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := data.Objects.Create(db); err != nil {
		log.Fatal(err)
	}

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

	wg.Add(1)
	go func() {
		log.WithError(fed.ListenAndServe(ctx, db, *addr, *cert, *key)).Error("Listener has failed")
		cancel()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		log.WithError(gem.ListenAndServe(ctx, db, *gemAddr, *gemCert, *gemKey)).Error("Listener has failed")
		cancel()
		wg.Done()
	}()

	<-ctx.Done()
	wg.Wait()
}
