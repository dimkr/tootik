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

package gemini

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"
)

const reqTimeout = time.Second * 30

func getUser(ctx context.Context, db *sql.DB, conn net.Conn, tlsConn *tls.Conn, log *slog.Logger) (*ap.Actor, error) {
	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		return nil, nil
	}

	clientCert := state.PeerCertificates[0]

	certHash := fmt.Sprintf("%x", sha256.Sum256(clientCert.Raw))

	id := ""
	actorString := ""
	if err := db.QueryRowContext(ctx, `select id, actor from persons where id like ? and certhash = ?`, fmt.Sprintf("https://%s/user/%%", cfg.Domain), certHash).Scan(&id, &actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, front.ErrNotRegistered
	} else if err != nil {
		return nil, fmt.Errorf("Failed to fetch user for %s: %w", certHash, err)
	}

	log.Debug("Found existing user", "hash", certHash, "user", id)

	actor := ap.Actor{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal %s: %w", id, err)
	}

	return &actor, nil
}

func handle(ctx context.Context, handler front.Handler, conn net.Conn, db *sql.DB, resolver *fed.Resolver, wg *sync.WaitGroup, log *slog.Logger) {
	if err := conn.SetDeadline(time.Now().Add(reqTimeout)); err != nil {
		log.Warn("Failed to set deadline", "error", err)
		return
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		log.Warn("Invalid connection")
		return
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		log.Warn("Handshake failed", "error", err)
		return
	}

	req := make([]byte, 1024+2)
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

		if total > 2 && req[total-2] == '\r' && req[total-1] == '\n' {
			break
		}
	}

	reqUrl, err := url.Parse(string(req[:total-2]))
	if err != nil {
		log.Warn("Failed to parse request", "request", string(req[:total-2]), "error", err)
		return
	}

	w := gmi.Wrap(conn)

	if reqUrl.Host != cfg.Domain {
		log.Info("Wrong host", "host", reqUrl.Host)
		w.Status(53, "Wrong host")
		return
	}

	user, err := getUser(ctx, db, conn, tlsConn, log)
	if err != nil && errors.Is(err, front.ErrNotRegistered) && reqUrl.Path == "/users" {
		log.Info("Redirecting new user")
		w.Redirect("/users/register")
		return
	} else if err != nil && !errors.Is(err, front.ErrNotRegistered) {
		log.Warn("Failed to get user", "error", err)
		w.Error()
		return
	}

	handler.Handle(ctx, log, w, reqUrl, user, db, resolver, wg)
}

func ListenAndServe(ctx context.Context, log *slog.Logger, db *sql.DB, handler front.Handler, resolver *fed.Resolver, addr, certPath, keyPath string) error {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}
	l, err := tls.Listen("tcp", addr, &config)
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
			requestCtx, cancelRequest := context.WithTimeout(ctx, reqTimeout)

			timer := time.AfterFunc(reqTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				handle(requestCtx, handler, conn, db, resolver, &wg, log)
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
