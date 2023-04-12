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
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/user"
	"io"
	"net/url"
	"regexp"
)

var userNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]{4,32}$`)

func init() {
	handlers[regexp.MustCompile(`^/users/register$`)] = register
}

func register(w io.Writer, r *request) {
	if r.User != nil {
		r.Log.Warn("Registered user cannot register again")
		fmt.Fprintf(w, "40 Already registered as %s\r\n", r.User.PreferredUsername)
		return
	}

	tlsConn, ok := w.(*tls.Conn)
	if !ok {
		r.Log.Error("Invalid connection")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		r.Log.Warn("No client certificate")
		w.Write([]byte("30 /users\r\n"))
		return
	}

	clientCert := state.PeerCertificates[0]
	certHash := fmt.Sprintf("%x", sha256.Sum256(clientCert.Raw))

	var taken int
	if err := r.QueryRow(`select exists (select 1 from persons where id like ? and actor->>'clientCertificate' = ?)`, fmt.Sprintf("https://%s/%%", cfg.Domain), certHash).Scan(&taken); err != nil {
		r.Log.WithField("hash", certHash).WithError(err).Warn("Failed to check if cerificate hash is already in use")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if taken == 1 {
		r.Log.WithField("hash", certHash).Warn("Cerificate hash is already in use")
		w.Write([]byte("40 Client certificate is already in use\r\n"))
		return
	}

	userName := clientCert.Subject.CommonName

	if r.URL.RawQuery != "" {
		altName, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			r.Log.WithField("query", r.URL.RawQuery).WithError(err).Info("Failed to decode user name")
			w.Write([]byte("40 Bad input\r\n"))
			return
		}
		if altName != "" {
			userName = altName
		}
	}

	if userName == "" {
		w.Write([]byte("10 New user name\r\n"))
		return
	}

	if !userNameRegex.MatchString(userName) {
		fmt.Fprintf(w, "10 %s is invalid, enter user name\r\n", userName)
		return
	}

	if err := r.QueryRow(`select exists (select 1 from persons where id = ?)`, fmt.Sprintf("https://%s/user/%s", cfg.Domain, userName)).Scan(&taken); err != nil {
		r.Log.WithField("name", userName).WithError(err).Warn("Failed to check if username is taken")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if taken == 1 {
		r.Log.WithField("name", userName).Warn("Username is already taken")
		fmt.Fprintf(w, "10 %s already taken, enter user name\r\n", userName)
		return
	}

	r.Log.WithField("name", userName).Info("Creating new user")

	if _, err := user.Create(r.Context, r.DB, fmt.Sprintf("https://%s/user/%s", cfg.Domain, userName), userName, certHash); err != nil {
		r.Log.WithField("name", userName).WithError(err).Warn("Failed to create new user")
		w.Write([]byte("40 Failed to create new user\r\n"))
		return
	}

	w.Write([]byte("30 /users\r\n"))
}
