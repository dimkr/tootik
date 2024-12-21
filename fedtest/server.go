/*
Copyright 2024 Dima Krasner

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

package fedtest

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	Test      *testing.T
	Domain    string
	Config    *cfg.Config
	DB        *sql.DB
	Resolver  ap.Resolver
	NobodyKey httpsig.Key
	Frontend  gemini.Listener
	Backend   http.Handler
	Incoming  *inbox.Queue
	Outgoing  *fed.Queue

	listener, tlsListener net.Listener
	socketPath            string
	dbPath                string
}

const (
	maxRedirects = 5
)

var (
	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=localhost.localdomain:8965" -out cert.pem -keyout key.pem -days 3650
	*/
	serverCert = `-----BEGIN CERTIFICATE-----
MIIBlTCCATugAwIBAgIUOGTIsA9mqhGNYtYpbQOGyQT0nFMwCgYIKoZIzj0EAwIw
IDEeMBwGA1UEAwwVbG9jYWxob3N0LmxvY2FsZG9tYWluMB4XDTIzMTEwMTA2NTMz
MFoXDTMzMTAyOTA2NTMzMFowIDEeMBwGA1UEAwwVbG9jYWxob3N0LmxvY2FsZG9t
YWluMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEVNlOzZ9KfKeYYiqZNXnoV/fQ
9k+DfRqxWInuK5y7ELr3Pr2eYvfu7Xaf8QGjbGQxL3u2JzdtaNCfPzN41Ek2ZKNT
MFEwHQYDVR0OBBYEFPTIrvyjUvvDj9My+0UJegEpWrWoMB8GA1UdIwQYMBaAFPTI
rvyjUvvDj9My+0UJegEpWrWoMA8GA1UdEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwID
SAAwRQIhAJYpAxqES3+AEJicrYv+vpyvfEd9eMBHrubIE1TUAtKqAiBz1kzKzmiQ
MrE9j+LF/+UCOfJBrRiimWzvo6f3wpFdMQ==
-----END CERTIFICATE-----`

	serverKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgLY7FG8+1FoNtWhqM
yxCtWh+7hNUTnM3I8YVOLo8KkyuhRANCAARU2U7Nn0p8p5hiKpk1eehX99D2T4N9
GrFYie4rnLsQuvc+vZ5i9+7tdp/xAaNsZDEve7YnN21o0J8/M3jUSTZk
-----END PRIVATE KEY-----`
)

func NewServer(ctx context.Context, t *testing.T, domain string, client fed.Client) *Server {
	var cfg cfg.Config

	cfg.FillDefaults()
	cfg.MinActorAge = 0
	cfg.RegistrationInterval = 0
	cfg.UserNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]{3,32}$`)
	cfg.PostThrottleUnit = 0
	cfg.EditThrottleUnit = 0
	cfg.MinActorEditInterval = 0
	cfg.ResolverCacheTTL = 0
	cfg.ResolverRetryInterval = 0

	dbPath := filepath.Join(t.TempDir(), domain+".sqlite3")

	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?%s", dbPath, cfg.DatabaseOptions))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	resolver := fed.NewResolver(nil, domain, &cfg, client, db)

	if err := migrations.Run(ctx, domain, db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	_, nobodyKey, err := user.CreateNobody(ctx, domain, db)
	if err != nil {
		t.Fatalf("Failed to run create the nobody user: %v", err)
	}

	handler, err := front.NewHandler(domain, false, &cfg, resolver, db)
	if err != nil {
		t.Fatalf("Failed to run create a Handler: %v", err)
	}

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	if err != nil {
		t.Fatalf("Failed to run create the Gemini server keypair: %v", err)
	}

	backend, err := (&fed.Listener{
		Domain:   domain,
		Closed:   false,
		Config:   &cfg,
		DB:       db,
		ActorKey: nobodyKey,
		Resolver: resolver,
	}).NewHandler()
	if err != nil {
		t.Fatalf("Failed to run create the federation handler: %v", err)
	}

	socketPath := fmt.Sprintf("/tmp/%s-%s.socket", t.Name(), domain)
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		ClientAuth:   tls.RequestClientCert,
	}

	tlsListener := tls.NewListener(listener, &serverCfg)

	return &Server{
		Test:      t,
		Domain:    domain,
		Config:    &cfg,
		DB:        db,
		Resolver:  resolver,
		NobodyKey: nobodyKey,
		Frontend: gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			DB:      db,
			Handler: handler,
		},
		Backend: backend,
		Incoming: &inbox.Queue{
			Domain:   domain,
			Config:   &cfg,
			DB:       db,
			Resolver: resolver,
			Key:      nobodyKey,
		},
		Outgoing: &fed.Queue{
			Domain:   domain,
			Config:   &cfg,
			DB:       db,
			Resolver: resolver,
		},
		listener:    listener,
		tlsListener: tlsListener,
		socketPath:  socketPath,
		dbPath:      dbPath,
	}
}

func (s *Server) Stop() {
	s.DB.Close()
	os.Remove(s.dbPath)

	s.tlsListener.Close()

	s.listener.Close()
	os.Remove(s.socketPath)
}

func (s *Server) handle(cert tls.Certificate, path, input string, redirects int) Page {
	if redirects == maxRedirects {
		s.Test.Fatal("Too many redirects")
	}

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	clientConn, err := net.Dial("unix", s.socketPath)
	if err != nil {
		s.Test.Fatalf("Failed to connect: %v", err)
	}
	defer clientConn.Close()

	serverTlsConn, err := s.tlsListener.Accept()
	if err != nil {
		s.Test.Fatalf("Failed to accept connection: %v", err)
	}
	defer serverTlsConn.Close()

	clientTlsConn := tls.Client(clientConn, &clientCfg)
	defer clientTlsConn.Close()

	c := make(chan error)
	go func() {
		c <- clientTlsConn.Handshake()
	}()
	go func() {
		c <- serverTlsConn.(*tls.Conn).Handshake()
	}()
	for i := 0; i < 2; i++ {
		if err := <-c; err != nil {
			s.Test.Fatalf("Failed to perform TLS handshake: %v", err)
		}
	}

	if input == "" {
		_, err = fmt.Fprintf(clientTlsConn, "gemini://%s%s\r\n", s.Domain, path)
	} else {
		_, err = fmt.Fprintf(clientTlsConn, "gemini://%s%s?%s\r\n", s.Domain, path, url.QueryEscape(input))
	}
	if err != nil {
		s.Test.Fatalf("Failed to send request: %v", err)
	}

	s.Frontend.Handle(context.Background(), serverTlsConn)

	serverTlsConn.Close()

	resp, err := io.ReadAll(clientTlsConn)
	if err != nil {
		s.Test.Fatalf("Failed to read response: %v", err)
	}

	if _, err := s.Outgoing.ProcessBatch(context.Background()); err != nil {
		s.Test.Fatalf("Failed to process outgoing activities queue: %v", err)
	}

	prased := parseResponse(s, cert, path, string(resp))

	if strings.HasPrefix(prased.Status, "30 ") {
		return s.handle(cert, prased.Status[3:len(resp)-2], "", redirects+1)
	}

	return prased
}

// HandleInput is like [Server.Handle] but also accepts user-provided input.
func (s *Server) HandleInput(cert tls.Certificate, path, input string) Page {
	return s.handle(cert, path, input, 0)
}

// Handle simulates a Gemini request, follows redirects and returns a [Page].
func (s *Server) Handle(cert tls.Certificate, path string) Page {
	return s.handle(cert, path, "", 0)
}

func (s *Server) Register(cert tls.Certificate) Page {
	return s.Handle(cert, "/users/register").OK()
}
