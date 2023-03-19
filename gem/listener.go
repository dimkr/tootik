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

package gem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/go-ap/activitypub"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

const reqTimeout = time.Second * 30

var handlers = map[*regexp.Regexp]func(context.Context, io.Writer, *url.URL, []string, *data.Object, *sql.DB){}

func handle(ctx context.Context, conn net.Conn, db *sql.DB) {
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(reqTimeout)); err != nil {
		log.WithError(err).Warn("Failed to set deadline")
		return
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		log.Warn("Invalid connection")
		return
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		log.WithError(err).Warn("Handshake failed")
		return
	}

	state := tlsConn.ConnectionState()

	req := make([]byte, 2048)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil {
			log.WithError(err).Warn("Failed to receive request")
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
		log.WithError(err).Warnf("Failed to parse request: %s", string(req[:total-2]))
		return
	}

	if reqUrl.Path == "/" {
		conn.Write([]byte("30 /public\r\n"))
		return
	}

	var user *data.Object

	if len(state.PeerCertificates) > 0 {
		clientCert := state.PeerCertificates[0]
		name := clientCert.Subject.CommonName

		if name == "Public" {
			conn.Write([]byte("40 Error\r\n"))
			return
		}

		id := fmt.Sprintf("https://%s/user/%s", cfg.Domain, name)
		hash := fmt.Sprintf("%x", sha256.Sum256(clientCert.Raw))

		user, err = data.Objects.GetByID(id, db)
		if err == nil {
			m := map[string]any{}
			if err := json.Unmarshal([]byte(user.Object), &m); err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to parse user: %w", err)
				return
			}

			v, ok := m["clientCertificate"]
			if !ok {
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			s, ok := v.(string)
			if !ok {
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			if s != hash {
				conn.Write([]byte("61 CN is already in use\r\n"))
				return
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			cmd := exec.CommandContext(ctx, "openssl", "genrsa", "2048")
			stdout := bytes.Buffer{}
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to generate key: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			priv, err := ioutil.ReadAll(&stdout)
			if err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to generate key: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			cmd = exec.CommandContext(ctx, "openssl", "rsa", "-pubout")
			cmd.Stdin = bytes.NewBuffer(priv)
			stdout = bytes.Buffer{}
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to generate key: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			pub, err := ioutil.ReadAll(&stdout)
			if err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to generate key: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			body, err := json.Marshal(
				map[string]any{
					"@context": []string{
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1",
					},
					"id":                id,
					"type":              "Person",
					"preferredUsername": name,
					"inbox":             fmt.Sprintf("https://%s/inbox/%s", cfg.Domain, name),
					"outbox":            fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, name),
					"followers":         fmt.Sprintf("https://%s/followers/%s", cfg.Domain, name),
					"publicKey": map[string]any{
						"id":           fmt.Sprintf("https://%s/user/%s#main-key", cfg.Domain, name),
						"owner":        fmt.Sprintf("https://%s/user/%s", cfg.Domain, name),
						"publicKeyPem": string(pub),
					},
					"manuallyApprovesFollowers": false,

					"privateKey":        string(priv),
					"clientCertificate": hash,
				})
			if err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to generate key: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}

			user = &data.Object{
				ID:     id,
				Type:   string(activitypub.PersonType),
				Object: string(body),
			}
			if err := data.Objects.Insert(db, user); err != nil {
				log.WithField("user", id).WithError(err).Warn("Failed to insert new user: %w", err)
				conn.Write([]byte("40 Error\r\n"))
				return
			}
		} else {
			log.WithField("user", id).WithError(err).Warn("Failed to fetch user: %w", err)
			conn.Write([]byte("40 Error\r\n"))
			return
		}
	}

	for re, handler := range handlers {
		matches := re.FindStringSubmatch(reqUrl.Path)
		if len(matches) > 0 {
			handler(ctx, conn, reqUrl, matches, user, db)
			return
		}
	}

	if user == nil {
		conn.Write([]byte("30 /public\r\n"))
	} else {
		conn.Write([]byte("30 /users\r\n"))
	}
}

func ListenAndServe(ctx context.Context, db *sql.DB, addr, certPath, keyPath string) error {
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
				log.WithError(err).Warn("Failed to accept a connection")
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

			go func() {
				<-requestCtx.Done()
				conn.Close()
			}()

			go func() {
				handle(requestCtx, conn, db)
				cancelRequest()
			}()
		}
	}

	wg.Wait()
	return nil
}
