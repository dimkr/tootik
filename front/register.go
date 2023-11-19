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
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/user"
	"net/url"
	"regexp"
	"time"
)

const registrationInterval = time.Hour

var userNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]{4,32}$`)

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
	if err := r.QueryRow(`select exists (select 1 from persons where host = ? and certhash = ?)`, cfg.Domain, certHash).Scan(&taken); err != nil {
		r.Log.Warn("Failed to check if cerificate hash is already in use", "hash", certHash, "error", err)
		w.Error()
		return
	}

	if taken == 1 {
		r.Log.Warn("Cerificate hash is already in use", "hash", certHash)
		w.Status(40, "Client certificate is already in use")
		return
	}

	userName := clientCert.Subject.CommonName

	if r.URL.RawQuery != "" {
		altName, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			r.Log.Info("Failed to decode user name", "query", r.URL.RawQuery, "error", err)
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
		r.Log.Warn("Failed to check if username is taken", "name", userName, "error", err)
		w.Error()
		return
	}

	if taken == 1 {
		r.Log.Warn("Username is already taken", "name", userName)
		w.Statusf(10, "%s is already taken, enter user name", userName)
		return
	}

	var lastRegister sql.NullInt64
	if err := r.QueryRow(`select max(inserted) from persons where host = ?`, cfg.Domain).Scan(&lastRegister); err != nil {
		r.Log.Warn("Failed to check last registration time", "name", userName, "error", err)
		w.Error()
		return
	}

	if lastRegister.Valid {
		elapsed := time.Since(time.Unix(lastRegister.Int64, 0))
		if elapsed < registrationInterval {
			w.Statusf(40, "Registration is closed for %s", (registrationInterval - elapsed).Truncate(time.Second).String())
			return
		}
	}

	r.Log.Info("Creating new user", "name", userName)

	if _, err := user.Create(r.Context, r.DB, fmt.Sprintf("https://%s/user/%s", cfg.Domain, userName), userName, certHash); err != nil {
		r.Log.Warn("Failed to create new user", "name", userName, "error", err)
		w.Status(40, "Failed to create new user")
		return
	}

	w.Redirect("/users")
}
