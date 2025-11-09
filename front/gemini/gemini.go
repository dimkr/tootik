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

// Package gemini exposes a Gemini interface.
package gemini

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/httpsig"
)

type Listener struct {
	Domain   string
	Config   *cfg.Config
	DB       *sql.DB
	Handler  front.Handler
	Addr     string
	CertPath string
	KeyPath  string
	Buffers  sync.Pool
}

func (gl *Listener) getUser(ctx context.Context, tlsConn *tls.Conn) (*ap.Actor, [2]httpsig.Key, error) {
	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		return nil, [2]httpsig.Key{}, nil
	}

	clientCert := state.PeerCertificates[0]

	if time.Now().After(clientCert.NotAfter) {
		return nil, [2]httpsig.Key{}, nil
	}

	certHash := fmt.Sprintf("%X", sha256.Sum256(clientCert.Raw))

	var rsaPrivKeyPem, ed25519PrivKeyMultibase string
	var actor ap.Actor
	var approved int
	if err := gl.DB.QueryRowContext(ctx, `select json(persons.actor), persons.rsaprivkey, persons.ed25519privkey, certificates.approved from certificates join persons on persons.actor->>'$.preferredUsername' = certificates.user where persons.host = ? and certificates.hash = ? and certificates.expires > unixepoch()`, gl.Domain, certHash).Scan(&actor, &rsaPrivKeyPem, &ed25519PrivKeyMultibase, &approved); err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, [2]httpsig.Key{}, front.ErrNotRegistered
	} else if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to fetch user for %s: %w", certHash, err)
	}

	if approved == 0 {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to fetch user for %s: %w", certHash, front.ErrNotApproved)
	}

	rsaPrivKey, err := data.ParseRSAPrivateKey(rsaPrivKeyPem)
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to parse RSA private key for %s: %w", certHash, err)
	}

	ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to decode Ed15519 private key for %s: %w", certHash, err)
	}

	slog.Debug("Found existing user", "hash", certHash, "user", actor.ID)
	return &actor, [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey},
	}, nil
}

func (gl *Listener) readRequest(ctx context.Context, conn net.Conn) *front.Request {
	req := gl.Buffers.Get().([]byte)
	defer gl.Buffers.Put(req)

	total := 0
	for {
		n, err := conn.Read(req[total : total+1])
		if err != nil && total == 0 && errors.Is(err, io.EOF) {
			slog.Debug("Failed to receive request", "error", err)
			return nil
		} else if err != nil && !errors.Is(err, io.EOF) {
			slog.Warn("Failed to receive request", "error", err)
			return nil
		}
		if n <= 0 {
			slog.Warn("Failed to receive request")
			return nil
		}
		total += n

		if total == cap(req) {
			slog.Warn("Request is too big")
			return nil
		}

		if total > 2 && req[total-2] == '\r' && req[total-1] == '\n' {
			break
		}
	}

	r := &front.Request{
		Context: ctx,
		Body:    conn,
	}

	var err error
	r.URL, err = url.Parse(string(req[:total-2]))
	if err != nil {
		slog.Warn("Failed to parse request", "request", string(req[:total-2]), "error", err)
		return nil
	}

	return r
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

	r := gl.readRequest(ctx, conn)
	if r == nil {
		return
	}

	w := gmi.Wrap(conn)
	defer w.Flush()

	var err error
	r.User, r.Keys, err = gl.getUser(ctx, tlsConn)
	if err != nil && errors.Is(err, front.ErrNotRegistered) && r.URL.Path == "/users" {
		slog.Info("Redirecting new user")
		w.Redirect("/users/register")
		return
	} else if errors.Is(err, front.ErrNotApproved) {
		w.Status(40, "Client certificate is awaiting approval")
		return
	} else if err != nil && !errors.Is(err, front.ErrNotRegistered) {
		slog.Warn("Failed to get user", "error", err)
		w.Error()
		return
	} else if err == nil && r.User == nil && r.URL.Path == "/users" {
		w.Status(60, "Client certificate required")
		return
	} else if r.User == nil && gl.Config.RequireRegistration && r.URL.Path != "/" && r.URL.Path != "/help" && r.URL.Path != "/users/register" {
		w.Status(40, "Must register first")
		return
	}

	if r.User == nil {
		r.Log = slog.With(slog.Group("request", "path", r.URL.Path))
	} else {
		r.Log = slog.With(slog.Group("request", "path", r.URL.Path, "user", r.User.PreferredUsername))
	}

	gl.Handler.Handle(r, w)
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

	wg.Go(func() {
		<-ctx.Done()
		l.Close()
	})

	conns := make(chan net.Conn)

	wg.Go(func() {
		for ctx.Err() == nil {
			conn, err := l.Accept()
			if err != nil {
				slog.Warn("Failed to accept a connection", "error", err)
				continue
			}

			conns <- conn
		}
	})

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case conn := <-conns:
			requestCtx, cancelRequest := context.WithTimeout(ctx, gl.Config.GeminiRequestTimeout)

			timer := time.AfterFunc(gl.Config.GeminiRequestTimeout, cancelRequest)

			wg.Go(func() {
				<-requestCtx.Done()
				conn.Close()
			})

			wg.Go(func() {
				gl.Handle(requestCtx, conn)
				timer.Stop()
				cancelRequest()
			})
		}
	}

	wg.Wait()
	return nil
}
