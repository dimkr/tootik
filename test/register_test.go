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

package test

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	"github.com/stretchr/testify/assert"
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

	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=erin" -out cert.pem -keyout key.pem -days 3650
	*/
	erinCert = `-----BEGIN CERTIFICATE-----
MIIBczCCARmgAwIBAgIUb+Vb5itzgaYGeKectZ/R0vTl4dIwCgYIKoZIzj0EAwIw
DzENMAsGA1UEAwwEZXJpbjAeFw0yMzExMDEwNzMzNDRaFw0zMzEwMjkwNzMzNDRa
MA8xDTALBgNVBAMMBGVyaW4wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARlOXQM
XCHmESk7Zzor+JhUFjIwf/vM/KbIgszEUXJ7ccMctWelADHc5dJjrf/nbXhvf/NA
y0ibEVc6KM97dRMPo1MwUTAdBgNVHQ4EFgQUG0V5VrS+Wj3U3A4j2tgdJKTYmn8w
HwYDVR0jBBgwFoAUG0V5VrS+Wj3U3A4j2tgdJKTYmn8wDwYDVR0TAQH/BAUwAwEB
/zAKBggqhkjOPQQDAgNIADBFAiEA84fpaDdh6JRaT4R7qGdhfO9zYnJ6VcQEnAiN
eXPo8z4CIBkYBJV6O6+Pvmjxs6SAa6c8SVb4Q72lJQpe0woYdmXf
-----END CERTIFICATE-----`

	erinKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgwpYr2U3MW256QQjk
7hUgeic6Imr/bzK7gcwLkQ6Sc4uhRANCAARlOXQMXCHmESk7Zzor+JhUFjIwf/vM
/KbIgszEUXJ7ccMctWelADHc5dJjrf/nbXhvf/NAy0ibEVc6KM97dRMP
-----END PRIVATE KEY-----`

	erinCertHash = "EB108D8A0EF3051B2830FEAA87AA79ADA1C5B60DD8A7CECCC8EC25BD086DC11A"

	erinOtherCert = `-----BEGIN CERTIFICATE-----
MIIBdDCCARmgAwIBAgIUbFZSevby3dlfix1x1rSPo97pmwEwCgYIKoZIzj0EAwIw
DzENMAsGA1UEAwwEZXJpbjAeFw0yNDEyMTExNzAzMDJaFw0zNDEyMDkxNzAzMDJa
MA8xDTALBgNVBAMMBGVyaW4wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS1Kb1E
6KuqOe+7igiQ5A6P5xL26TRaGEA5K95MyP1+0tNsd+JkjBazHstMmJn02HVfFCmE
59xK4lMFLDlnQQJVo1MwUTAdBgNVHQ4EFgQUymF2MzroPIh9yRRfvW70d078ZjMw
HwYDVR0jBBgwFoAUymF2MzroPIh9yRRfvW70d078ZjMwDwYDVR0TAQH/BAUwAwEB
/zAKBggqhkjOPQQDAgNJADBGAiEAuBkYquBho1QVLFnhXn4E8SW89IpdRhJlchCO
AE2Vgr0CIQDXm/PrYlc/oGnUL4HDL44RS8Sz3Hecb098uAIACKlghw==
-----END CERTIFICATE-----`

	erinOtherKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgFuJwYfaV8bUaZEDc
GBjiDGYOI7qKaHNQd6X2uXvrM0KhRANCAAS1Kb1E6KuqOe+7igiQ5A6P5xL26TRa
GEA5K95MyP1+0tNsd+JkjBazHstMmJn02HVfFCmE59xK4lMFLDlnQQJV
-----END PRIVATE KEY-----`

	erinOtherCertHash = "4561F7655D75BC965B6204AAED6CB1CE2047E1CA79397A81B41AC26281D42EFF"

	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=david" -out cert.pem -keyout key.pem -days 3650
	*/
	davidCert = `-----BEGIN CERTIFICATE-----
MIIBdDCCARugAwIBAgIUEN8F95Gx0Aea4jbONtJtrFVYjBowCgYIKoZIzj0EAwIw
EDEOMAwGA1UEAwwFZGF2aWQwHhcNMjMxMTAxMDgxMTEzWhcNMzMxMDI5MDgxMTEz
WjAQMQ4wDAYDVQQDDAVkYXZpZDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABId5
Vy+MFm/KjtCgRGoW0lYcT4q3EAnUvfj3Qg0LnQN42YaAlrhKvUMHZ52BpYQK0DFI
X7laKw6O/cUANgm2Kw2jUzBRMB0GA1UdDgQWBBSFlyVsYu+R1RcUHCH8TV2fUAlX
zTAfBgNVHSMEGDAWgBSFlyVsYu+R1RcUHCH8TV2fUAlXzTAPBgNVHRMBAf8EBTAD
AQH/MAoGCCqGSM49BAMCA0cAMEQCIHkaUT5KHy+kkvMryZGUnkX1oqB/e/lvFFg7
uSGGclcgAiA1eqHRDwFml4oSVDi3sMF9OtVXqp/ktJLsarP/IZ81jA==
-----END CERTIFICATE-----`

	davidKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgA9Tox1/NjVs3av3C
f7Kgs5SdkJkcanCS3Ibes8Tsm+ChRANCAASHeVcvjBZvyo7QoERqFtJWHE+KtxAJ
1L3490INC50DeNmGgJa4Sr1DB2edgaWECtAxSF+5WisOjv3FADYJtisN
-----END PRIVATE KEY-----`
)

func TestRegister_RedirectNoCertificate(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("60 Client certificate required\r\n", string(resp))
}

func TestRegister_InvitationRequired(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	cfg.RequireInvitation = true

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users/invitations/accept\r\n", string(resp))
}

func TestRegister_InvitationPrompt(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	cfg.RequireInvitation = true

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/invitations/accept\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("10 Invitation code\r\n", string(resp))
}

func TestRegister_InvalidInvitationCode(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	cfg.RequireInvitation = true

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/invitations/accept?abc\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("40 Invalid invitation code\r\n", string(resp))
}

func TestRegister_Redirect(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users/register\r\n", string(resp))
}

func TestRegister_NoCertificate(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users\r\n", string(resp))
}

func TestRegister_HappyFlow(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users\r\n", string(resp))
}

func TestRegister_AlreadyRegistered(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.RegistrationInterval = 0

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
	assert.NoError(err)

	_, _, err = user.Create(context.Background(), domain, db, &cfg, "erin", ap.Person, erinKeyPair.Leaf)
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("40 Already registered as erin\r\n", string(resp))
}

func TestRegister_Twice(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.RegistrationInterval = 0

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	for _, expected := range []string{
		"30 /users\r\n",
		"40 Already registered as erin\r\n",
	} {

		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, &clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(tlsReader.Handshake())
		})
		wg.Go(func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		})
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
		assert.NoError(err)

		l := gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			Handler: handler,
			DB:      db,
		}
		l.Handle(context.Background(), tlsWriter)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Equal(expected, string(resp))
	}
}

func TestRegister_Throttling(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	erinCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	davidKeyPair, err := tls.X509KeyPair([]byte(davidCert), []byte(davidKey))
	assert.NoError(err)

	davidCfg := tls.Config{
		Certificates:       []tls.Certificate{davidKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	for _, data := range []struct {
		clientCfg *tls.Config
		pattern   string
	}{
		{&erinCfg, "^30 /users\r\n$"},
		{&davidCfg, "^40 Registration is closed for .+\r\n$"},
	} {
		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, data.clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(tlsReader.Handshake())
		})
		wg.Go(func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		})
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
		assert.NoError(err)

		l := gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			Handler: handler,
			DB:      db,
		}
		l.Handle(context.Background(), tlsWriter)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.pattern, string(resp))
	}
}

func TestRegister_Throttling30Minutes(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	erinCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	davidKeyPair, err := tls.X509KeyPair([]byte(davidCert), []byte(davidKey))
	assert.NoError(err)

	davidCfg := tls.Config{
		Certificates:       []tls.Certificate{davidKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	for _, data := range []struct {
		clientCfg *tls.Config
		pattern   string
	}{
		{&erinCfg, "^30 /users\r\n$"},
		{&davidCfg, "^40 Registration is closed for .+\r\n$"},
	} {
		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, data.clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(tlsReader.Handshake())
		})
		wg.Go(func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		})
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
		assert.NoError(err)

		l := gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			Handler: handler,
			DB:      db,
		}
		l.Handle(context.Background(), tlsWriter)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.pattern, string(resp))

		_, err = db.Exec(`update certificates set inserted = unixepoch() - 1800`)
		assert.NoError(err)
	}
}

func TestRegister_Throttling1Hour(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	erinCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	davidKeyPair, err := tls.X509KeyPair([]byte(davidCert), []byte(davidKey))
	assert.NoError(err)

	davidCfg := tls.Config{
		Certificates:       []tls.Certificate{davidKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	for _, data := range []struct {
		clientCfg *tls.Config
		pattern   string
	}{
		{&erinCfg, "^30 /users\r\n$"},
		{&davidCfg, "^30 /users\r\n$"},
	} {
		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, data.clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(tlsReader.Handshake())
		})
		wg.Go(func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		})
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
		assert.NoError(err)

		l := gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			Handler: handler,
			DB:      db,
		}
		l.Handle(context.Background(), tlsWriter)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.pattern, string(resp))

		_, err = db.Exec(`update certificates set inserted = unixepoch() - 3600`)
		assert.NoError(err)
	}
}

func TestRegister_TwoCertificates(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.RegistrationInterval = 0

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	erinOtherKeyPair, err := tls.X509KeyPair([]byte(erinOtherCert), []byte(erinOtherKey))
	assert.NoError(err)

	otherClientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinOtherKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	for _, data := range []struct {
		url       string
		expected  string
		clientCfg *tls.Config
	}{
		{"gemini://localhost.localdomain:8965/users\r\n", "^30 /users/register\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^30 /users\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^20 text/gemini\r\n.+", &clientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^40 Already registered as erin\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^30 /users/register\r\n$", &otherClientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^30 /users\r\n$", &otherClientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^40 Client certificate is awaiting approval\r\n$", &otherClientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^40 Client certificate is awaiting approval\r\n$", &otherClientCfg},
		{fmt.Sprintf("gemini://localhost.localdomain:8965/users/certificates/approve/%s\r\n", erinOtherCertHash), "^30 /users/certificates\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^40 Already registered as erin\r\n$", &otherClientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^20 text/gemini\r\n.+", &otherClientCfg},
		{fmt.Sprintf("gemini://localhost.localdomain:8965/users/certificates/revoke/%s\r\n", erinCertHash), "^30 /users/certificates\r\n$", &otherClientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^30 /users/register\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users/register?generate\r\n", "^30 /users\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^40 Client certificate is awaiting approval\r\n$", &clientCfg},
		{"gemini://localhost.localdomain:8965/users\r\n", "^20 text/gemini\r\n.+", &otherClientCfg},
	} {
		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, data.clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(tlsReader.Handshake())
		})
		wg.Go(func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		})
		wg.Wait()

		_, err = tlsReader.Write([]byte(data.url))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
		assert.NoError(err)

		l := gemini.Listener{
			Domain:  domain,
			Config:  &cfg,
			Handler: handler,
			DB:      db,
		}
		l.Handle(context.Background(), tlsWriter)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.expected, string(resp))
	}
}

func TestRegister_ForbiddenUserName(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.CompiledForbiddenUserNameRegex = regexp.MustCompile(`^(root|localhost|erin.*|ip6-.*|.*(admin|tootik).*)$`)

	assert.NoError(migrations.Run(context.Background(), domain, db))

	serverKeyPair, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	assert.NoError(err)

	serverCfg := tls.Config{
		Certificates: []tls.Certificate{serverKeyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}

	erinKeyPair, err := tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	assert.NoError(err)

	clientCfg := tls.Config{
		Certificates:       []tls.Certificate{erinKeyPair},
		InsecureSkipVerify: true,
	}

	socketPath := fmt.Sprintf("/tmp/%s.socket", t.Name())

	localListener, err := net.Listen("unix", socketPath)
	assert.NoError(err)
	defer os.Remove(socketPath)

	tlsListener := tls.NewListener(localListener, &serverCfg)
	defer tlsListener.Close()

	unixReader, err := net.Dial("unix", socketPath)
	assert.NoError(err)
	defer unixReader.Close()

	tlsWriter, err := tlsListener.Accept()
	assert.NoError(err)

	tlsReader := tls.Client(unixReader, &clientCfg)
	defer tlsReader.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(tlsReader.Handshake())
	})
	wg.Go(func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
	})
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register?generate\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, &inbox.Inbox{})
	assert.NoError(err)

	l := gemini.Listener{
		Domain:  domain,
		Config:  &cfg,
		Handler: handler,
		DB:      db,
	}
	l.Handle(context.Background(), tlsWriter)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("40 Forbidden user name\r\n", string(resp))
}
