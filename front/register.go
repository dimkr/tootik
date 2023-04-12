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

package front

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/text"
	"github.com/dimkr/tootik/user"
	"net/url"
	"regexp"
)

var userNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]{4,32}$`)

func init() {
	handlers[regexp.MustCompile(`^/users/register$`)] = register
}

func register(w text.Writer, r *request) {
	if r.User != nil {
		r.Log.Warn("Registered user cannot register again")
		w.Statusf(40, "Already registered as %s", r.User.PreferredUsername)
		return
	}

	tlsConn, ok := w.Unwrap().(*tls.Conn)
	if !ok {
		r.Log.Error("Invalid connection")
		w.Error()
		return
	}

	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) == 0 {
		r.Log.Warn("No client certificate")
		w.Redirect("/users")
		return
	}

	clientCert := state.PeerCertificates[0]
	certHash := fmt.Sprintf("%x", sha256.Sum256(clientCert.Raw))

	var taken int
	if err := r.QueryRow(`select exists (select 1 from persons where id like ? and actor->>'clientCertificate' = ?)`, fmt.Sprintf("https://%s/%%", cfg.Domain), certHash).Scan(&taken); err != nil {
		r.Log.WithField("hash", certHash).WithError(err).Warn("Failed to check if cerificate hash is already in use")
		w.Error()
		return
	}

	if taken == 1 {
		r.Log.WithField("hash", certHash).Warn("Cerificate hash is already in use")
		w.Status(40, "Client certificate is already in use")
		return
	}

	userName := clientCert.Subject.CommonName

	if r.URL.RawQuery != "" {
		altName, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			r.Log.WithField("query", r.URL.RawQuery).WithError(err).Info("Failed to decode user name")
			w.Status(40, "Bad input")
			return
		}
		if altName != "" {
			userName = altName
		}
	}

	if userName == "" {
		w.Status(10, "New user name")
		return
	}

	if !userNameRegex.MatchString(userName) {
		w.Statusf(10, "%s is invalid, enter user name", userName)
		return
	}

	if err := r.QueryRow(`select exists (select 1 from persons where id = ?)`, fmt.Sprintf("https://%s/user/%s", cfg.Domain, userName)).Scan(&taken); err != nil {
		r.Log.WithField("name", userName).WithError(err).Warn("Failed to check if username is taken")
		w.Error()
		return
	}

	if taken == 1 {
		r.Log.WithField("name", userName).Warn("Username is already taken")
		w.Statusf(10, "%s is already taken, enter user name", userName)
		return
	}

	r.Log.WithField("name", userName).Info("Creating new user")

	if _, err := user.Create(r.Context, r.DB, fmt.Sprintf("https://%s/user/%s", cfg.Domain, userName), userName, certHash); err != nil {
		r.Log.WithField("name", userName).WithError(err).Warn("Failed to create new user")
		w.Status(40, "Failed to create new user")
		return
	}

	w.Redirect("/users")
}
