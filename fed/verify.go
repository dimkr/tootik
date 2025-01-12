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

package fed

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

func (l *Listener) verify(r *http.Request, body []byte, flags ap.ResolverFlag) (*ap.Actor, error) {
	sig, err := httpsig.Extract(r, body, l.Domain, time.Now(), l.Config.MaxRequestAge)
	if err != nil {
		return nil, fmt.Errorf("failed to verify message: %w", err)
	}

	actor, err := l.Resolver.ResolveID(r.Context(), l.ActorKey, sig.KeyID, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify message: %w", sig.KeyID, err)
	}

	publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
	if err != nil {
		publicKey, err = x509.ParsePKCS1PublicKey(publicKeyPem.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
		}
	}

	if err := sig.Verify(publicKey); err != nil {
		return nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
	}

	return actor, nil
}
