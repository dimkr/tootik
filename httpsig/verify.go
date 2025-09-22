/*
Copyright 2024, 2025 Dima Krasner

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

package httpsig

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Signature struct {
	KeyID     string
	Alg       string
	s         string
	signature []byte
}

const (
	minKeyBits = 2048
	maxKeyBits = 8192
)

var (
	signatureAttrRegex = regexp.MustCompile(`\b([^"=]+)="([^"]+)"`)

	defaultRequiredComponents          = []string{"@target-uri"}
	defaultRequiredComponentsWithQuery = []string{"@target-uri", "@query"}

	requiredPostComponents          = []string{"@target-uri", "content-digest"}
	requiredPostComponentsWithQuery = []string{"@target-uri", "@query", "content-digest"}

	rsaAlgorithms = map[string]struct{}{
		"":                {},
		"rsa-sha256":      {},
		"hs2019":          {},
		"rsa-v1_5-sha256": {},
	}
)

// Extract extracts signature attributes, validates them and returns a [Signature].
// Caller should obtain the key and pass it to [Signature.Verify].
// It supports RFC9421 and falls back to draft-cavage-http-signatures.
func Extract(r *http.Request, body []byte, domain string, now time.Time, maxAge time.Duration) (*Signature, error) {
	input := r.Header.Values("Signature-Input")
	if len(input) == 1 {
		required := defaultRequiredComponents

		if r.Method == http.MethodPost && r.URL.RawQuery != "" {
			required = requiredPostComponentsWithQuery
		} else if r.Method == http.MethodPost {
			required = requiredPostComponents
		} else if r.URL.RawQuery != "" {
			required = defaultRequiredComponentsWithQuery
		}

		return rfc9421Extract(r, input[0], body, domain, now, maxAge, required)
	} else if len(input) > 1 {
		return nil, errors.New("more than one Signature-Input")
	}

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

	if now.Sub(t) > maxAge {
		return nil, errors.New("date is too old")
	}
	if t.Sub(now) > maxAge {
		return nil, errors.New("date is too new")
	}

	values := r.Header.Values("Signature")
	if len(values) > 1 {
		return nil, errors.New("more than one signature")
	} else if len(values) == 0 {
		return nil, errors.New("no signature")
	}

	var keyID, headers, signature, algorithm string
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
			if algorithm != "" {
				return nil, errors.New("more than one algorithm")
			}

			algorithm = m[2]

			if algorithm != "rsa-sha256" && algorithm != "hs2019" {
				return nil, errors.New("unsupported algorithm: " + algorithm)
			}

		default:
			return nil, errors.New("unsupported attribute: " + m[1])
		}
	}

	if keyID == "" || headers == "" || signature == "" {
		return nil, errors.New("invalid signature header: " + values[0])
	}

	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	rawHeaders := strings.Fields(strings.ToLower(headers))
	uniqueHeaders := make(map[string]struct{}, len(rawHeaders))
	for _, h := range rawHeaders {
		if _, dup := uniqueHeaders[h]; dup {
			return nil, errors.New("duplicate header: " + h)
		}
		uniqueHeaders[h] = struct{}{}
	}

	if len(rawHeaders) == 0 {
		return nil, errors.New("empty headers list")
	}

	if _, ok := uniqueHeaders["(request-target)"]; !ok {
		return nil, errors.New("(request-target) is not signed")
	}

	if _, ok := uniqueHeaders["host"]; !ok {
		return nil, errors.New("host is not signed")
	}

	if _, ok := uniqueHeaders["date"]; !ok {
		return nil, errors.New("date is not signed")
	}

	if body != nil {
		if _, ok := uniqueHeaders["digest"]; !ok {
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

	s, err := buildSignatureString(r, rawHeaders)
	if err != nil {
		return nil, err
	}

	return &Signature{
		KeyID:     keyID,
		Alg:       algorithm,
		s:         s,
		signature: rawSignature,
	}, nil
}

// Verify verifies a signature.
func (s *Signature) Verify(key any) error {
	switch v := key.(type) {
	case *rsa.PublicKey:
		if _, ok := rsaAlgorithms[s.Alg]; !ok {
			return errors.New("alg is not RSA")
		}

		bits := v.N.BitLen()
		if bits < minKeyBits || bits > maxKeyBits {
			return fmt.Errorf("invalid RSA key size: %d", bits)
		}

		sigBits := len(s.signature) * 8
		if sigBits != bits {
			return fmt.Errorf("invalid RSA signature size: %d", sigBits)
		}

		hash := sha256.Sum256([]byte(s.s))
		if err := rsa.VerifyPKCS1v15(v, crypto.SHA256, hash[:], s.signature); err != nil {
			return fmt.Errorf("invalid RSA signature: %w", err)
		}

	case ed25519.PublicKey:
		if s.Alg != "" && s.Alg != "ed25519" {
			return errors.New("alg is not Ed25519: " + s.Alg)
		}

		if len(s.signature) != ed25519.SignatureSize {
			return fmt.Errorf("invalid signature size: %d", len(s.signature))
		}

		if !ed25519.Verify(v, []byte(s.s), s.signature) {
			return errors.New("invalid ed25519 signature")
		}

	default:
		return fmt.Errorf(`cannot verify alg="%s" with %T`, s.Alg, key)
	}

	return nil
}
