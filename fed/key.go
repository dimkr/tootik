/*
Copyright 2023, 2024 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless ruired by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fed

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"github.com/dimkr/tootik/ap"
)

// key is used to sign outgoing HTTP requests.
// The key fetched from the database when used for the first time.
type key struct {
	loaded      bool
	publicKeyID string
	privateKey  any
}

func (k *key) Load(ctx context.Context, db *sql.DB, actor *ap.Actor) (string, any, error) {
	if k.loaded {
		return k.publicKeyID, k.privateKey, nil
	}

	var publicKeyID, privateKeyPemString string
	if err := db.QueryRowContext(ctx, `select actor->>'$.publicKey.id', privkey from persons where id = ?`, actor.ID).Scan(&publicKeyID, &privateKeyPemString); err != nil {
		return "", nil, fmt.Errorf("failed to fetch key for %s: %w", actor.ID, err)
	}

	privateKeyPem, _ := pem.Decode([]byte(privateKeyPemString))

	privateKey, err := x509.ParsePKCS8PrivateKey(privateKeyPem.Bytes)
	if err != nil {
		// fallback for openssl<3.0.0
		privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyPem.Bytes)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse key for %s: %w", actor.ID, err)
		}
	}

	k.loaded = true
	k.publicKeyID = publicKeyID
	k.privateKey = privateKey

	return publicKeyID, privateKey, nil
}
