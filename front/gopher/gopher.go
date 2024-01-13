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

package gopher

import (
	"context"
	"database/sql"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmap"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"
)

func handle(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, conn net.Conn, handler front.Handler, db *sql.DB, resolver *fed.Resolver, wg *sync.WaitGroup) {
	if err := conn.SetDeadline(time.Now().Add(cfg.GopherRequestTimeout)); err != nil {
		log.Warn("Failed to set deadline", "error", err)
		return
	}

	req := make([]byte, 128)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil {
			log.Warn("Failed to receive request", "error", err)
			return
		}
		if n <= 0 {
			log.Warn("Failed to receive request")
			return
		}
		total += n

		if total == cap(req) {
			log.Warn("Request is too big")
			return
		}

		if total >= 2 && req[total-2] == '\r' && req[total-1] == '\n' {
			break
		}
	}

	path := string(req[:total-2])
	if path == "" {
		path = "/"
	}

	reqUrl, err := url.Parse(path)
	if err != nil {
		log.Warn("Failed to parse request", "path", path, "error", err)
		return
	}

	w := gmap.Wrap(conn, domain, cfg)

	handler.Handle(ctx, log.With(slog.Group("request", "path", reqUrl.Path)), w, reqUrl, nil, db, resolver, wg)
}

func ListenAndServe(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, handler front.Handler, db *sql.DB, resolver *fed.Resolver, addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		<-ctx.Done()
		l.Close()
		wg.Done()
	}()

	conns := make(chan net.Conn)

	wg.Add(1)
	go func() {
		for ctx.Err() == nil {
			conn, err := l.Accept()
			if err != nil {
				log.Warn("Failed to accept a connection", "error", err)
				continue
			}

			conns <- conn
		}
		wg.Done()
	}()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case conn := <-conns:
			requestCtx, cancelRequest := context.WithTimeout(ctx, cfg.GopherRequestTimeout)

			timer := time.AfterFunc(cfg.GopherRequestTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				handle(requestCtx, domain, cfg, log, conn, handler, db, resolver, &wg)
				conn.Write([]byte(".\r\n"))
				conn.Close()
				timer.Stop()
				cancelRequest()
				wg.Done()
			}()
		}
	}

	wg.Wait()
	return nil
}
