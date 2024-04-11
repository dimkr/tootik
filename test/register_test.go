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

package test

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/gemini"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/migrations"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
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

func TestRegister_Redirect(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
	wg.Add(2)
	go func() {
		assert.NoError(tlsReader.Handshake())
		wg.Done()
	}()
	go func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		wg.Done()
	}()
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, false, &cfg)
	assert.NoError(err)

	l := gemini.Listener{
		Domain:   domain,
		Config:   &cfg,
		Handler:  handler,
		DB:       db,
		Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
		Log:      slog.Default(),
	}
	l.Handle(context.Background(), tlsWriter, &wg)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users/register\r\n", string(resp))
}

func TestRegister_HappyFlow(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
	wg.Add(2)
	go func() {
		assert.NoError(tlsReader.Handshake())
		wg.Done()
	}()
	go func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		wg.Done()
	}()
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, false, &cfg)
	assert.NoError(err)

	l := gemini.Listener{
		Domain:   domain,
		Config:   &cfg,
		Handler:  handler,
		DB:       db,
		Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
		Log:      slog.Default(),
	}
	l.Handle(context.Background(), tlsWriter, &wg)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("30 /users\r\n", string(resp))
}

func TestRegister_HappyFlowRegistrationClosed(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
	wg.Add(2)
	go func() {
		assert.NoError(tlsReader.Handshake())
		wg.Done()
	}()
	go func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		wg.Done()
	}()
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
	assert.NoError(err)

	handler, err := front.NewHandler(domain, true, &cfg)
	assert.NoError(err)

	l := gemini.Listener{
		Domain:   domain,
		Config:   &cfg,
		Handler:  handler,
		DB:       db,
		Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
		Log:      slog.Default(),
	}
	l.Handle(context.Background(), tlsWriter, &wg)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("40 Registration is closed\r\n", string(resp))
}

func TestRegister_AlreadyRegistered(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
	wg.Add(2)
	go func() {
		assert.NoError(tlsReader.Handshake())
		wg.Done()
	}()
	go func() {
		assert.NoError(tlsWriter.(*tls.Conn).Handshake())
		wg.Done()
	}()
	wg.Wait()

	_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
	assert.NoError(err)

	_, _, err = user.Create(context.Background(), domain, db, "erin", "e")
	assert.NoError(err)

	handler, err := front.NewHandler(domain, false, &cfg)
	assert.NoError(err)

	l := gemini.Listener{
		Domain:   domain,
		Config:   &cfg,
		Handler:  handler,
		DB:       db,
		Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
		Log:      slog.Default(),
	}
	l.Handle(context.Background(), tlsWriter, &wg)

	tlsWriter.Close()

	resp, err := io.ReadAll(tlsReader)
	assert.NoError(err)

	assert.Equal("10 erin is already taken, enter user name\r\n", string(resp))
}

func TestRegister_Twice(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
		wg.Add(2)
		go func() {
			assert.NoError(tlsReader.Handshake())
			wg.Done()
		}()
		go func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
			wg.Done()
		}()
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, false, &cfg)
		assert.NoError(err)

		l := gemini.Listener{
			Domain:   domain,
			Config:   &cfg,
			Handler:  handler,
			DB:       db,
			Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
			Log:      slog.Default(),
		}
		l.Handle(context.Background(), tlsWriter, &wg)

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

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
		wg.Add(2)
		go func() {
			assert.NoError(tlsReader.Handshake())
			wg.Done()
		}()
		go func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
			wg.Done()
		}()
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, false, &cfg)
		assert.NoError(err)

		l := gemini.Listener{
			Domain:   domain,
			Config:   &cfg,
			Handler:  handler,
			DB:       db,
			Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
			Log:      slog.Default(),
		}
		l.Handle(context.Background(), tlsWriter, &wg)

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

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
		wg.Add(2)
		go func() {
			assert.NoError(tlsReader.Handshake())
			wg.Done()
		}()
		go func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
			wg.Done()
		}()
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, false, &cfg)
		assert.NoError(err)

		l := gemini.Listener{
			Domain:   domain,
			Config:   &cfg,
			Handler:  handler,
			DB:       db,
			Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
			Log:      slog.Default(),
		}
		l.Handle(context.Background(), tlsWriter, &wg)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.pattern, string(resp))

		_, err = db.Exec(`update persons set inserted = unixepoch() - 1800`)
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

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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
		wg.Add(2)
		go func() {
			assert.NoError(tlsReader.Handshake())
			wg.Done()
		}()
		go func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
			wg.Done()
		}()
		wg.Wait()

		_, err = tlsReader.Write([]byte("gemini://localhost.localdomain:8965/users/register\r\n"))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, false, &cfg)
		assert.NoError(err)

		l := gemini.Listener{
			Domain:   domain,
			Config:   &cfg,
			Handler:  handler,
			DB:       db,
			Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
			Log:      slog.Default(),
		}
		l.Handle(context.Background(), tlsWriter, &wg)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.pattern, string(resp))

		_, err = db.Exec(`update persons set inserted = unixepoch() - 3600`)
		assert.NoError(err)
	}
}

func TestRegister_RedirectTwice(t *testing.T) {
	assert := assert.New(t)

	dbPath := fmt.Sprintf("/tmp/%s.sqlite3?_journal_mode=WAL", t.Name())
	defer os.Remove(fmt.Sprintf("/tmp/%s.sqlite3", t.Name()))
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(err)

	var cfg cfg.Config
	cfg.FillDefaults()

	assert.NoError(migrations.Run(context.Background(), slog.Default(), domain, db))

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

	for _, data := range []struct {
		url      string
		expected string
	}{
		{"gemini://localhost.localdomain:8965/users\r\n", "^30 /users/register\r\n$"},
		{"gemini://localhost.localdomain:8965/users/register\r\n", "^30 /users\r\n$"},
		{"gemini://localhost.localdomain:8965/users\r\n", "^20 text/gemini\r\n.+"},
		{"gemini://localhost.localdomain:8965/users/register\r\n", "^40 Already registered as erin\r\n$"},
	} {
		unixReader, err := net.Dial("unix", socketPath)
		assert.NoError(err)
		defer unixReader.Close()

		tlsWriter, err := tlsListener.Accept()
		assert.NoError(err)

		tlsReader := tls.Client(unixReader, &clientCfg)
		defer tlsReader.Close()

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			assert.NoError(tlsReader.Handshake())
			wg.Done()
		}()
		go func() {
			assert.NoError(tlsWriter.(*tls.Conn).Handshake())
			wg.Done()
		}()
		wg.Wait()

		_, err = tlsReader.Write([]byte(data.url))
		assert.NoError(err)

		handler, err := front.NewHandler(domain, false, &cfg)
		assert.NoError(err)

		l := gemini.Listener{
			Domain:   domain,
			Config:   &cfg,
			Handler:  handler,
			DB:       db,
			Resolver: fed.NewResolver(nil, domain, &cfg, &http.Client{}),
			Log:      slog.Default(),
		}
		l.Handle(context.Background(), tlsWriter, &wg)

		tlsWriter.Close()

		resp, err := io.ReadAll(tlsReader)
		assert.NoError(err)

		assert.Regexp(data.expected, string(resp))
	}
}
