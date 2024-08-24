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

// Package gopher exposes a limited Gopher interface.
package gopher

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmap"
	"github.com/dimkr/tootik/httpsig"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"
)

type Listener struct {
	Domain   string
	Config   *cfg.Config
	Log      *slog.Logger
	Handler  front.Handler
	DB       *sql.DB
	Resolver ap.Resolver
	Addr     string
}

const bufferSize = 256

func (gl *Listener) handle(ctx context.Context, conn net.Conn, wg *sync.WaitGroup) {
	if err := conn.SetDeadline(time.Now().Add(gl.Config.GopherRequestTimeout)); err != nil {
		gl.Log.Warn("Failed to set deadline", "error", err)
		return
	}

	req := make([]byte, 128)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil && total == 0 && errors.Is(err, io.EOF) {
			gl.Log.Debug("Failed to receive request", "error", err)
			return
		} else if err != nil {
			gl.Log.Warn("Failed to receive request", "error", err)
			return
		}
		if n <= 0 {
			gl.Log.Warn("Failed to receive request")
			return
		}
		total += n

		if total == cap(req) {
			gl.Log.Warn("Request is too big")
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
		gl.Log.Warn("Failed to parse request", "path", path, "error", err)
		return
	}

	buffered := bufio.NewWriterSize(conn, bufferSize)
	defer buffered.Flush()
	w := gmap.Wrap(buffered, gl.Domain, gl.Config)

	gl.Handler.Handle(ctx, gl.Log.With(slog.Group("request", "path", reqUrl.Path)), nil, w, reqUrl, nil, httpsig.Key{}, gl.DB, gl.Resolver, wg)
}

// ListenAndServe handles Gopher requests.
func (gl *Listener) ListenAndServe(ctx context.Context) error {
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
				gl.Log.Warn("Failed to accept a connection", "error", err)
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
				gl.handle(requestCtx, conn, &wg)
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
