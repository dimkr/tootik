/*
Copyright 2026 Dima Krasner

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

package front

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"os"
	osuser "os/user"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	connectTimeout   = 30 * time.Second
	maxResponseSize  = 16 * 1024 * 1024
	defaultPort      = "1965"
	maxRedirects     = 5
	maxPermRedirects = 5
	certYears        = 10
)

func generateClientCert(user string) ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), nil)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: new(big.Int),
		Subject: pkix.Name{
			CommonName: user,
		},
		NotBefore:   now,
		NotAfter:    now.AddDate(certYears, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

func loadClientCert(ctx context.Context, db *sql.DB, address, user string) (tls.Certificate, error) {
	tx, err := db.BeginTx(ctx, nil)
	defer tx.Rollback()

	var certPEM, keyPEM []byte
	if err := tx.QueryRowContext(ctx, `select cert, key from certs where address = ?`, address).Scan(&certPEM, &keyPEM); err == nil {
		return tls.X509KeyPair(certPEM, keyPEM)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return tls.Certificate{}, err
	}

	certPEM, keyPEM, err = generateClientCert(user)
	if err != nil {
		return tls.Certificate{}, err
	}

	if _, err := tx.ExecContext(ctx, `insert into certs(address, cert, key) values(?, ?, ?)`, address, certPEM, keyPEM); err != nil {
		return tls.Certificate{}, err
	}

	if err := tx.Commit(); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPEM, keyPEM)
}

type geminiClient struct {
	db   *sql.DB
	cert tls.Certificate
}

func (c *geminiClient) verifyServer(ctx context.Context, hostport string, conn *tls.Conn) error {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("%s presented no certificate", hostport)
	}

	digest := fmt.Sprintf("%X", sha256.Sum256(state.PeerCertificates[0].Raw))

	var known string
	if err := c.db.QueryRowContext(ctx, `select hash from tofu where host = ?`, hostport).Scan(&known); errors.Is(err, sql.ErrNoRows) {
		_, err := c.db.ExecContext(ctx, `insert into tofu(host, hash) values(?, ?)`, hostport, digest)
		return err
	} else if err != nil {
		return err
	}

	if known != digest {
		return fmt.Errorf("remote certificate for %s changed", hostport)
	}

	return nil
}

func (c *geminiClient) effectiveURL(ctx context.Context, u *url.URL) (*url.URL, error) {
	for range maxPermRedirects {
		var target string
		if err := c.db.QueryRowContext(
			ctx,
			`select target from redirects where source = ?`,
			u.String(),
		).Scan(&target); errors.Is(err, sql.ErrNoRows) {
			return u, nil
		} else if err != nil {
			return nil, err
		}

		next, err := url.Parse(target)
		if err != nil {
			return nil, err
		}

		u = next
	}

	return u, nil
}

func (c *geminiClient) savePermanentRedirect(ctx context.Context, u *url.URL, resp string) error {
	status := resp
	if before, _, ok := strings.Cut(resp, "\r\n"); ok {
		status = before
	}

	if !strings.HasPrefix(status, "31 ") {
		return nil
	}

	rel, err := url.Parse(strings.TrimSpace(status[3:]))
	if err != nil {
		return nil
	}

	_, err = c.db.ExecContext(
		ctx,
		`insert into redirects(source, target) values(?, ?) on conflict(source) do update set target = excluded.target`,
		u.String(),
		u.ResolveReference(rel).String(),
	)
	return err
}

func (c *geminiClient) request(ctx context.Context, u *url.URL) (*url.URL, string, error) {
	u, err := c.effectiveURL(ctx, u)
	if err != nil {
		return nil, "", err
	}

	port := u.Port()
	if port == "" {
		port = defaultPort
	}
	hostport := net.JoinHostPort(u.Hostname(), port)

	deadline := time.Now().Add(connectTimeout)

	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	dialer := tls.Dialer{
		Config: &tls.Config{
			Certificates:       []tls.Certificate{c.cert},
			ServerName:         u.Hostname(),
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return nil, "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return nil, "", err
	}

	tlsConn := conn.(*tls.Conn)
	if err := c.verifyServer(ctx, hostport, tlsConn); err != nil {
		return nil, "", err
	}

	if _, err := fmt.Fprintf(conn, "%s\r\n", u); err != nil {
		return nil, "", err
	}

	resp, err := io.ReadAll(io.LimitReader(conn, maxResponseSize))
	if err != nil {
		return nil, "", err
	}

	body := string(resp)
	if err := c.savePermanentRedirect(ctx, u, body); err != nil {
		return nil, "", err
	}

	return u, body, nil
}

func Connect(ctx context.Context, db *sql.DB, user, host string, port int, path, input string) error {
	if user == "" {
		current, err := osuser.Current()
		if err != nil {
			return fmt.Errorf("failed to determine current user: %w", err)
		}
		user = current.Username
	}
	if host == "" {
		host = "localhost"
	}

	portStr := strconv.Itoa(port)
	hostport := net.JoinHostPort(host, portStr)

	cert, err := loadClientCert(ctx, db, user+"@"+hostport, user)
	if err != nil {
		return err
	}

	urlHost := host
	if portStr != defaultPort {
		urlHost = hostport
	}
	if path == "" {
		path = "/"
	}
	u := &url.URL{Scheme: "gemini", Host: urlHost, Path: path}
	if input != "" {
		u.RawQuery = input
	}

	c := &geminiClient{db: db, cert: cert}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		for i := 0; ; i++ {
			eff, resp, err := c.request(ctx, u)
			if err != nil {
				return err
			}

			status := resp
			if before, _, ok := strings.Cut(resp, "\r\n"); ok {
				status = before
			}

			if i < maxRedirects && (strings.HasPrefix(status, "30 ") || strings.HasPrefix(status, "31 ")) {
				rel, err := url.Parse(strings.TrimSpace(status[3:]))
				if err != nil {
					return err
				}

				u = eff.ResolveReference(rel)
				continue
			}

			_, err = os.Stdout.WriteString(resp)
			return err
		}
	}

	return repl(ctx, hostport, u, c.request)
}
