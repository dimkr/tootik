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
	"github.com/dimkr/tootik/data"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Signature struct {
	KeyID     string
	s         string
	signature []byte
}

const (
	minKeyBits = 2048
	maxKeyBits = 8192
)

var signatureAttrRegex = regexp.MustCompile(`\b([^"=]+)="([^"]+)"`)

// Extract extracts signature attributes and returns a [Signature].
// Caller should obtain the key and pass it to [Signature.Verify].
func Extract(r *http.Request, body []byte, domain string, maxAge time.Duration) (*Signature, error) {
	host := r.Header.Get("Host")
	if host == "" {
		if r.Host == "" {
			return nil, errors.New("host is unspecified")
		}

		if r.Host != domain {
			return nil, errors.New("wrong host: " + r.Host)
		}

		r.Header.Set("Host", r.Host)
	} else if host != domain {
		return nil, errors.New("wrong host: " + host)
	}

	date := r.Header.Get("Date")
	if date == "" {
		return nil, errors.New("date is unspecified")
	}

	t, err := time.Parse(http.TimeFormat, date)
	if err != nil {
		return nil, err
	}

	if time.Since(t) > maxAge {
		return nil, errors.New("date is too old")
	}

	values := r.Header.Values("Signature")
	if len(values) > 1 {
		return nil, errors.New("more than one signature")
	}

	var keyID, headers, signature string
	for _, m := range signatureAttrRegex.FindAllStringSubmatch(values[0], -1) {
		switch m[1] {
		case "keyId":
			if keyID != "" {
				return nil, errors.New("more than one keyId")
			}
			keyID = m[2]
		case "headers":
			if headers != "" {
				return nil, errors.New("more than one headers")
			}
			headers = m[2]
		case "signature":
			if signature != "" {
				return nil, errors.New("more than one signature")
			}
			signature = m[2]
		case "algorithm":
			continue
		default:
			return nil, errors.New("unsupported atribute: " + m[1])
		}
	}

	if keyID == "" || headers == "" || signature == "" {
		return nil, fmt.Errorf("invalid signature header: " + values[0])
	}

	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	rawHeaders := data.OrderedMap[string, struct{}]{}
	for _, h := range strings.Fields(strings.TrimSpace(strings.ToLower(headers))) {
		if rawHeaders.Contains(h) {
			return nil, errors.New("duplicate header: " + h)
		}
		rawHeaders.Store(strings.TrimSpace(h), struct{}{})
	}

	if len(rawHeaders) == 0 {
		return nil, errors.New("empty headers list")
	}

	if !rawHeaders.Contains("(request-target)") {
		return nil, errors.New("(request-target) is not signed")
	}

	if !rawHeaders.Contains("host") {
		return nil, errors.New("host is not signed")
	}

	if !rawHeaders.Contains("date") {
		return nil, errors.New("date is not signed")
	}

	if body != nil {
		if !rawHeaders.Contains("digest") {
			return nil, errors.New("digest is not signed")
		}

		digest := r.Header.Get("Digest")
		if digest == "" {
			return nil, errors.New("digest is unspecified")
		}

		if len(digest) <= len("SHA-256=") || !strings.HasPrefix(digest, "SHA-256=") {
			return nil, errors.New("invalid digest algorithm: " + digest)
		}

		rawDigest, err := base64.StdEncoding.DecodeString(digest[len("SHA-256="):])
		if err != nil {
			return nil, fmt.Errorf("invalid digest: %w", err)
		}

		if len(rawDigest) != sha256.Size {
			return nil, errors.New("invalid digest size")
		}

		hash := sha256.Sum256(body)
		if !bytes.Equal(hash[:], rawDigest) {
			return nil, errors.New("digest mismatch")
		}
	}

	s, err := buildSignatureString(r, rawHeaders.Keys())
	if err != nil {
		return nil, err
	}

	return &Signature{
		KeyID:     keyID,
		s:         s,
		signature: rawSignature,
	}, nil
}

// Verify verifies a signature.
func (s *Signature) Verify(key any) error {
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return errors.New("invalid public key")
	}

	bits := rsaKey.N.BitLen()
	if bits < minKeyBits || bits > maxKeyBits {
		return fmt.Errorf("invalid key size: %d", bits)
	}

	hash := sha256.Sum256([]byte(s.s))
	if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, hash[:], s.signature); err != nil {
		return err
	}

	return nil
}
