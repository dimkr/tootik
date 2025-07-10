/*
Copyright 2025 Dima Krasner

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
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dimkr/tootik/data"
)

var (
	defaultComponents     = []string{"@method", "@target-uri"}
	defaultPostComponents = []string{"@method", "@target-uri", "content-type", "content-digest"}

	signatureInputAttrRegex = regexp.MustCompile(`\b([^=;]+)=([^;]+)`)
)

func buildSignatureBase(r *http.Request, params string, components []string) (string, error) {
	var b strings.Builder

	for _, c := range components {
		switch c {
		case "@method":
			b.WriteString(`"@method": `)
			b.WriteString(r.Method)

		case "@target-uri":
			b.WriteString(`"@target-uri": `)
			b.WriteString(r.URL.String())

		case "@path":
			b.WriteString(`"@path": `)
			b.WriteString(r.URL.Path)

		case "@authority":
			b.WriteString(`"@authority": `)
			b.WriteString(r.URL.Host)

		default:
			if c[0] == '@' {
				return "", errors.New("unsupported component: " + c)
			}
			b.WriteByte('"')
			b.WriteString(strings.ToLower(c))
			b.WriteByte('"')
			b.WriteByte(':')
			b.WriteByte(' ')
			values, ok := r.Header[textproto.CanonicalMIMEHeaderKey(c)]
			if !ok || len(values) == 0 {
				return "", errors.New("unspecified header: " + c)
			}
			for j, v := range values {
				b.WriteString(strings.TrimSpace(v))
				if j < len(values)-1 {
					b.WriteByte(',')
					b.WriteByte(' ')
				}
			}
		}

		b.WriteByte('\n')
	}

	b.WriteString(`"@signature-params": `)
	b.WriteString(params)

	return b.String(), nil
}

// SignRFC9421 adds a signature to an outgoing HTTP request.
func SignRFC9421(
	r *http.Request,
	key Key,
	now, expires time.Time,
	digestAlg, sigAlg string,
	components []string,
) error {
	if key.ID == "" {
		return errors.New("empty key ID")
	}

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		switch digestAlg {
		case "sha-256":
			hash := sha256.Sum256(body)
			r.Header.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(hash[:])+":")

		case "sha-512":
			hash := sha512.Sum512(body)
			r.Header.Set("Content-Digest", "sha-512=:"+base64.StdEncoding.EncodeToString(hash[:])+":")

		default:
			return errors.New("invalid digest: " + digestAlg)
		}

		if components == nil {
			components = defaultPostComponents
		}
	} else if components == nil {
		components = defaultComponents
	}

	var params strings.Builder
	params.WriteByte('(')
	for i, comp := range components {
		if i > 0 {
			params.WriteByte(' ')
		}

		params.WriteByte('"')
		params.WriteString(comp)
		params.WriteByte('"')
	}

	params.WriteString(`);`)

	params.WriteString(`created=`)
	params.WriteString(strconv.FormatInt(now.Unix(), 10))
	params.WriteString(`;keyid="`)
	params.WriteString(key.ID)
	params.WriteByte('"')

	if sigAlg != "" {
		params.WriteString(`;alg="`)
		params.WriteString(sigAlg)
		params.WriteString(`"`)
	}

	if expires != (time.Time{}) {
		params.WriteString(`;expires=`)
		params.WriteString(strconv.FormatInt(expires.Unix(), 10))
	}

	s, err := buildSignatureBase(r, params.String(), components)
	if err != nil {
		return err
	}

	var sig []byte
	switch v := key.PrivateKey.(type) {
	case *rsa.PrivateKey:
		hash := sha256.Sum256([]byte(s))
		sig, err = rsa.SignPKCS1v15(nil, v, crypto.SHA256, hash[:])

	case ed25519.PrivateKey:
		sig = ed25519.Sign(v, []byte(s))

	default:
		return errors.New("invalid private key")
	}

	if err != nil {
		return err
	}

	r.Header.Set("Signature-Input", `sig1=`+params.String())
	r.Header.Set("Signature", `sig1=:`+base64.StdEncoding.EncodeToString(sig)+":")

	return nil
}

func rfc9421Extract(
	r *http.Request,
	input string,
	body []byte,
	domain string,
	now time.Time,
	maxAge time.Duration,
	requiredComponents []string,
) (*Signature, error) {
	if r.URL.Host != domain {
		return nil, errors.New("wrong host: " + r.URL.Host)
	}

	sigs := r.Header.Values("Signature")
	if len(sigs) > 1 {
		return nil, errors.New("more than one Signature")
	}

	var keyID, components, created, expires string
	for _, m := range signatureInputAttrRegex.FindAllStringSubmatch(input, -1) {
		switch m[1] {
		case "keyid":
			if keyID != "" {
				return nil, errors.New("more than one keyid")
			}

			keyID = m[2]

			if keyID[0] != '"' || keyID[len(keyID)-1] != '"' {
				return nil, errors.New("keyid is not quoted")
			}

			keyID = keyID[1 : len(keyID)-1]

		case "created":
			if created != "" {
				return nil, errors.New("more than one created")
			}

			created = m[2]

		case "expires":
			if expires != "" {
				return nil, errors.New("more than one expires")
			}

			expires = m[2]

			expiresSec, err := strconv.ParseInt(expires, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", expires, err)
			}

			if now.After(time.Unix(expiresSec, 0)) {
				return nil, errors.New("expired")
			}

		case "alg":
			if m[2] != `"rsa-v1_5-sha256"` {
				return nil, errors.New("unsupported alg: " + m[2])
			}

		default:
			if components != "" {
				return nil, errors.New("more than one set of components")
			}

			components = m[2]

			if components[0] != '(' || components[len(components)-1] != ')' {
				return nil, errors.New("components is not parenthesized")
			}

			components = components[1 : len(components)-1]
		}
	}

	if keyID == "" || components == "" || created == "" {
		return nil, errors.New("invalid signature input: " + input)
	}

	createdSec, err := strconv.ParseInt(created, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", created, err)
	}

	t := time.Unix(createdSec, 0)

	if expires == "" && now.Sub(t) > maxAge {
		return nil, errors.New("date is too old")
	}
	if t.Sub(now) > maxAge {
		return nil, errors.New("date is too new")
	}

	sep := strings.IndexByte(sigs[0], '=')
	if sep == -1 {
		return nil, fmt.Errorf("no separator in signature %s", sigs[0])
	}

	if len(sigs[0]) < sep+2 {
		return nil, fmt.Errorf("invalid signature %s", sigs[0])
	}

	rawSignature, err := base64.StdEncoding.DecodeString(sigs[0][sep+2 : len(sigs[0])-1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature %s: %w", sigs[0], err)
	}

	uniqueComponents := data.OrderedMap[string, struct{}]{}
	for c := range strings.FieldsSeq(components) {
		if c[0] != '"' || c[len(c)-1] != '"' {
			return nil, errors.New("invalid component: " + c)
		}

		c = c[1 : len(c)-1]

		if _, dup := uniqueComponents[c]; dup {
			return nil, errors.New("duplicate component: " + c)
		}

		uniqueComponents.Store(c, struct{}{})
	}

	if len(uniqueComponents) == 0 {
		return nil, errors.New("empty components list")
	}

	for _, c := range requiredComponents {
		if !uniqueComponents.Contains(c) {
			return nil, errors.New(c + " is not signed")
		}
	}

	if body != nil {
		digests := r.Header.Values("Content-Digest")
		if len(digests) == 0 {
			return nil, errors.New("multiple Content-Digest values")
		} else if len(digests) > 1 {
			return nil, errors.New("multiple Content-Digest values")
		}

		digest := digests[0]
		if digest == "" {
			return nil, errors.New("Content-Digest is empty")
		}

		if len(digest) > len("sha-256=:") && strings.HasPrefix(digest, "sha-256=:") && digest[len(digest)-1] == ':' {
			rawDigest, err := base64.StdEncoding.DecodeString(digest[len("sha-256=:") : len(digest)-1])
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
		} else if len(digest) > len("sha-512=:") && strings.HasPrefix(digest, "sha-512=:") && digest[len(digest)-1] == ':' {
			rawDigest, err := base64.StdEncoding.DecodeString(digest[len("sha-512=:") : len(digest)-1])
			if err != nil {
				return nil, fmt.Errorf("invalid digest: %w", err)
			}

			if len(rawDigest) != sha512.Size {
				return nil, errors.New("invalid digest size")
			}

			hash := sha512.Sum512(body)
			if !bytes.Equal(hash[:], rawDigest) {
				return nil, errors.New("digest mismatch")
			}
		} else {
			return nil, errors.New("invalid digest algorithm: " + digest)
		}
	}

	// TODO
	inputSep := strings.IndexByte(input, '=')

	s, err := buildSignatureBase(r, input[inputSep+1:], uniqueComponents.CollectKeys())
	if err != nil {
		return nil, err
	}

	return &Signature{
		KeyID:     keyID,
		s:         s,
		signature: rawSignature,
	}, nil
}
