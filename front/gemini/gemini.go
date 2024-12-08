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

// Package gemini exposes a Gemini interface.
package gemini

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/httpsig"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"
)

type Listener struct {
	Config   *cfg.Config
	DB       *sql.DB
	Handler  front.Handler
	Addr     string
	CertPath string
	KeyPath  string
}

func (gl *Listener) getUser(ctx context.Context, tlsConn *tls.Conn) (*ap.Actor, httpsig.Key, error) {
	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		return nil, httpsig.Key{}, nil
	}

	clientCert := state.PeerCertificates[0]

	certHash := fmt.Sprintf("%x", sha256.Sum256(clientCert.Raw))

	var id, privKeyPem string
	var actor ap.Actor
	if err := gl.DB.QueryRowContext(ctx, `select persons.id, persons.actor, persons.privkey from certificates join persons on persons.id = certificates.user where certificates.hash = ?`, certHash).Scan(&id, &actor, &privKeyPem); err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, httpsig.Key{}, front.ErrNotRegistered
	} else if err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to fetch user for %s: %w", certHash, err)
	}

	privKey, err := data.ParsePrivateKey(privKeyPem)
	if err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to parse private key for %s: %w", certHash, err)
	}

	slog.Debug("Found existing user", "hash", certHash, "user", id)
	return &actor, httpsig.Key{ID: actor.PublicKey.ID, PrivateKey: privKey}, nil
}

// Handle handles a Gemini request.
func (gl *Listener) Handle(ctx context.Context, conn net.Conn) {
	if err := conn.SetDeadline(time.Now().Add(gl.Config.GeminiRequestTimeout)); err != nil {
		slog.Warn("Failed to set deadline", "error", err)
		return
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		slog.Warn("Invalid connection")
		return
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		slog.Warn("Handshake failed", "error", err)
		return
	}

	req := make([]byte, 1024+2)
	total := 0
	for {
		n, err := conn.Read(req[total : total+1])
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

		if total > 2 && req[total-2] == '\r' && req[total-1] == '\n' {
			break
		}
	}

	r := front.Request{
		Context: ctx,
		Body:    conn,
	}

	var err error
	r.URL, err = url.Parse(string(req[:total-2]))
	if err != nil {
		slog.Warn("Failed to parse request", "request", string(req[:total-2]), "error", err)
		return
	}

	w := gmi.Wrap(conn)
	defer w.Flush()

	r.User, r.Key, err = gl.getUser(ctx, tlsConn)
	if err != nil && errors.Is(err, front.ErrNotRegistered) && r.URL.Path == "/users" {
		slog.Info("Redirecting new user")
		w.Redirect("/users/register")
		return
	} else if err != nil && !errors.Is(err, front.ErrNotRegistered) {
		slog.Warn("Failed to get user", "error", err)
		w.Error()
		return
	} else if err == nil && r.User == nil && r.URL.Path == "/users" {
		w.Status(60, "Client certificate required")
		return
	}

	if r.User == nil {
		r.Log = slog.With(slog.Group("request", "path", r.URL.Path))
	} else {
		r.Log = slog.With(slog.Group("request", "path", r.URL.Path, "user", r.User.PreferredUsername))
	}

	gl.Handler.Handle(&r, w)
}

// ListenAndServe handles Gemini requests.
func (gl *Listener) ListenAndServe(ctx context.Context) error {
	cert, err := tls.LoadX509KeyPair(gl.CertPath, gl.KeyPath)
	if err != nil {
		return err
	}

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}
	l, err := tls.Listen("tcp", gl.Addr, &config)
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
			requestCtx, cancelRequest := context.WithTimeout(ctx, gl.Config.GeminiRequestTimeout)

			timer := time.AfterFunc(gl.Config.GeminiRequestTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				<-requestCtx.Done()
				conn.Close()
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				gl.Handle(requestCtx, conn)
				timer.Stop()
				cancelRequest()
				wg.Done()
			}()
		}
	}

	wg.Wait()
	return nil
}
