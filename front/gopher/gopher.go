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

// Package gopher exposes a limited Gopher interface.
package gopher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmap"
)

type Listener struct {
	Domain  string
	Config  *cfg.Config
	Handler front.Handler
	Addr    string
}

func (gl *Listener) handle(ctx context.Context, conn net.Conn) {
	if err := conn.SetDeadline(time.Now().Add(gl.Config.GopherRequestTimeout)); err != nil {
		slog.Warn("Failed to set deadline", "error", err)
		return
	}

	req := make([]byte, 128)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil && total == 0 && errors.Is(err, io.EOF) {
			slog.Debug("Failed to receive request", "error", err)
			return
		} else if err != nil {
			slog.Warn("Failed to receive request", "error", err)
			return
		}
		if n <= 0 {
			slog.Warn("Failed to receive request")
			return
		}
		total += n

		if total == cap(req) {
			slog.Warn("Request is too big")
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

	r := front.Request{
		Context: ctx,
		Body:    conn,
	}

	var err error
	r.URL, err = url.Parse(path)
	if err != nil {
		slog.Warn("Failed to parse request", "path", path, "error", err)
		return
	}

	r.Log = slog.With(slog.Group("request", "path", r.URL.Path))

	w := gmap.Wrap(conn, gl.Domain, gl.Config)
	defer w.Flush()

	gl.Handler.Handle(&r, w)
}

// ListenAndServe handles Gopher requests.
func (gl *Listener) ListenAndServe(ctx context.Context) error {
	if gl.Config.RequireRegistration {
		slog.Warn("Disabling the Gopher listener because registration is required")
		<-ctx.Done()
		return nil
	}

	l, err := net.Listen("tcp", gl.Addr)
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
				slog.Warn("Failed to accept a connection", "error", err)
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
			requestCtx, cancelRequest := context.WithTimeout(ctx, gl.Config.GopherRequestTimeout)

			timer := time.AfterFunc(gl.Config.GopherRequestTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				gl.handle(requestCtx, conn)
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
