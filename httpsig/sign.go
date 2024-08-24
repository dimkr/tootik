/*
Copyright 2024 Dima Krasner

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

package httpsig

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	defaultHeaders = []string{"(request-target)", "host", "date"}
	postHeaders    = []string{"(request-target)", "host", "date", "content-type", "digest"}
)

// Sign adds a signature to an outgoing HTTP request.
func Sign(r *http.Request, key Key, now time.Time) error {
	if key.ID == "" {
		return errors.New("empty key ID")
	}

	headers := defaultHeaders
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := sha256.Sum256(body)
		r.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]))

		headers = postHeaders
	}

	r.Header.Set("Date", now.UTC().Format(http.TimeFormat))
	r.Header.Set("Host", r.URL.Host)

	s, err := buildSignatureString(r, headers)
	if err != nil {
		return err
	}

	rsaKey, ok := key.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return errors.New("invalid private key")
	}

	hash := sha256.Sum256([]byte(s))
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return err
	}

	r.Header.Set(
		"Signature",
		fmt.Sprintf(
			`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
			key.ID,
			strings.Join(headers, " "),
			base64.StdEncoding.EncodeToString(sig),
		),
	)

	return nil
}
