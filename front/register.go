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

package front

import (
	"crypto/ed25519"
	"crypto/tls"
	"database/sql"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/user"
)

func (h *Handler) register(w text.Writer, r *Request, args ...string) {
	println("got key", r.URL.RawQuery)
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
	userName := clientCert.Subject.CommonName

	if time.Now().After(clientCert.NotAfter) {
		r.Log.Warn("Client certificate has expired", "name", userName, "expired", clientCert.NotAfter)
		w.Status(40, "Client certificate has expired")
		return
	}

	if userName == "" {
		w.Status(40, "Invalid user name")
		return
	}

	if !h.Config.CompiledUserNameRegex.MatchString(userName) {
		w.Status(40, "Invalid user name")
		return
	}

	if h.Config.CompiledForbiddenUserNameRegex.MatchString(userName) {
		w.Status(40, "Forbidden user name")
		return
	}

	var lastRegister sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from certificates`).Scan(&lastRegister); err != nil {
		r.Log.Warn("Failed to check last registration time", "name", userName, "error", err)
		w.Error()
		return
	}

	if lastRegister.Valid {
		elapsed := time.Since(time.Unix(lastRegister.Int64, 0))
		if elapsed < h.Config.RegistrationInterval {
			w.Statusf(40, "Registration is closed for %s", (h.Config.RegistrationInterval - elapsed).Truncate(time.Second).String())
			return
		}
	}

	r.Log.Info("Creating new user", "name", userName)

	switch r.URL.RawQuery {
	case "":
		w.Status(10, "Create nomadic user? (y/n)")
		return

	case "n":
		if _, _, err := user.Create(r.Context, h.Domain, h.DB, userName, ap.Person, clientCert); err != nil {
			r.Log.Warn("Failed to create new user", "name", userName, "error", err)
			w.Status(40, "Failed to create new user")
			return
		}

	case "y":
		w.Status(10, "base58-encoded Ed25519 private key")
		return

	default:
		if r.URL.RawQuery[0] != 'z' {
			w.Statusf(40, "Invalid key prefix: %c", r.URL.RawQuery[0])
			return
		}

		rawKey := base58.Decode(r.URL.RawQuery[1:])

		if len(rawKey) != ed25519.SeedSize+2 {
			w.Statusf(40, "Invalid key length: %c", len(rawKey))
			return
		}

		if rawKey[0] != 0x80 || rawKey[1] != 0x26 {
			w.Statusf(40, "Invalid key prefix: %02x%02x", rawKey[0], rawKey[1])
			return
		}

		if _, _, err := user.CreateNomadic(r.Context, h.Domain, h.DB, userName, clientCert, ed25519.NewKeyFromSeed(rawKey[2:])); err != nil {
			r.Log.Warn("Failed to create new nomadic user", "name", userName, "error", err)
			w.Status(40, "Failed to create new user")
			return
		}
	}

	w.Redirect("/users")
}
