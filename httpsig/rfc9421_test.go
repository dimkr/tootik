/*
Copyright 2025, 2026 Dima Krasner

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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
)

// B.1.4.  Example Ed25519 Test Key
const (
	ed25519PublicPem = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAJrQLj5P/89iXES9+vFgrIy29clF9CC/oPPsw3c5D0bs=
-----END PUBLIC KEY-----`

	ed25519PrivatePem = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIJ+DYvh6SEqVTm50DFtMDoQikTmiCqirVv9mWG9qfSnF
-----END PRIVATE KEY-----`

	// B.1.1.  Example RSA Key
	rsaPublicPem = `-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAhAKYdtoeoy8zcAcR874L8cnZxKzAGwd7v36APp7Pv6Q2jdsPBRrw
WEBnez6d0UDKDwGbc6nxfEXAy5mbhgajzrw3MOEt8uA5txSKobBpKDeBLOsdJKFq
MGmXCQvEG7YemcxDTRPxAleIAgYYRjTSd/QBwVW9OwNFhekro3RtlinV0a75jfZg
kne/YiktSvLG34lw2zqXBDTC5NHROUqGTlML4PlNZS5Ri2U4aCNx2rUPRcKIlE0P
uKxI4T+HIaFpv8+rdV6eUgOrB2xeI1dSFFn/nnv5OoZJEIB+VmuKn3DCUcCZSFlQ
PSXSfBDiUGhwOw76WuSSsf1D4b/vLoJ10wIDAQAB
-----END RSA PUBLIC KEY-----`

	rsaPrivatePem = `-----BEGIN RSA PRIVATE KEY-----
MIIEqAIBAAKCAQEAhAKYdtoeoy8zcAcR874L8cnZxKzAGwd7v36APp7Pv6Q2jdsP
BRrwWEBnez6d0UDKDwGbc6nxfEXAy5mbhgajzrw3MOEt8uA5txSKobBpKDeBLOsd
JKFqMGmXCQvEG7YemcxDTRPxAleIAgYYRjTSd/QBwVW9OwNFhekro3RtlinV0a75
jfZgkne/YiktSvLG34lw2zqXBDTC5NHROUqGTlML4PlNZS5Ri2U4aCNx2rUPRcKI
lE0PuKxI4T+HIaFpv8+rdV6eUgOrB2xeI1dSFFn/nnv5OoZJEIB+VmuKn3DCUcCZ
SFlQPSXSfBDiUGhwOw76WuSSsf1D4b/vLoJ10wIDAQABAoIBAG/JZuSWdoVHbi56
vjgCgkjg3lkO1KrO3nrdm6nrgA9P9qaPjxuKoWaKO1cBQlE1pSWp/cKncYgD5WxE
CpAnRUXG2pG4zdkzCYzAh1i+c34L6oZoHsirK6oNcEnHveydfzJL5934egm6p8DW
+m1RQ70yUt4uRc0YSor+q1LGJvGQHReF0WmJBZHrhz5e63Pq7lE0gIwuBqL8SMaA
yRXtK+JGxZpImTq+NHvEWWCu09SCq0r838ceQI55SvzmTkwqtC+8AT2zFviMZkKR
Qo6SPsrqItxZWRty2izawTF0Bf5S2VAx7O+6t3wBsQ1sLptoSgX3QblELY5asI0J
YFz7LJECgYkAsqeUJmqXE3LP8tYoIjMIAKiTm9o6psPlc8CrLI9CH0UbuaA2JCOM
cCNq8SyYbTqgnWlB9ZfcAm/cFpA8tYci9m5vYK8HNxQr+8FS3Qo8N9RJ8d0U5Csw
DzMYfRghAfUGwmlWj5hp1pQzAuhwbOXFtxKHVsMPhz1IBtF9Y8jvgqgYHLbmyiu1
mwJ5AL0pYF0G7x81prlARURwHo0Yf52kEw1dxpx+JXER7hQRWQki5/NsUEtv+8RT
qn2m6qte5DXLyn83b1qRscSdnCCwKtKWUug5q2ZbwVOCJCtmRwmnP131lWRYfj67
B/xJ1ZA6X3GEf4sNReNAtaucPEelgR2nsN0gKQKBiGoqHWbK1qYvBxX2X3kbPDkv
9C+celgZd2PW7aGYLCHq7nPbmfDV0yHcWjOhXZ8jRMjmANVR/eLQ2EfsRLdW69bn
f3ZD7JS1fwGnO3exGmHO3HZG+6AvberKYVYNHahNFEw5TsAcQWDLRpkGybBcxqZo
81YCqlqidwfeO5YtlO7etx1xLyqa2NsCeG9A86UjG+aeNnXEIDk1PDK+EuiThIUa
/2IxKzJKWl1BKr2d4xAfR0ZnEYuRrbeDQYgTImOlfW6/GuYIxKYgEKCFHFqJATAG
IxHrq1PDOiSwXd2GmVVYyEmhZnbcp8CxaEMQoevxAta0ssMK3w6UsDtvUvYvF22m
qQKBiD5GwESzsFPy3Ga0MvZpn3D6EJQLgsnrtUPZx+z2Ep2x0xc5orneB5fGyF1P
WtP+fG5Q6Dpdz3LRfm+KwBCWFKQjg7uTxcjerhBWEYPmEMKYwTJF5PBG9/ddvHLQ
EQeNC8fHGg4UXU8mhHnSBt3EA10qQJfRDs15M38eG2cYwB1PZpDHScDnDA0=
-----END RSA PRIVATE KEY-----`
)

var (
	rsaPrivate *rsa.PrivateKey
	rsaPublic  *rsa.PublicKey

	ed25519Private any
	ed25519Public  any
)

func init() {
	rsaPrivateKeyPem, _ := pem.Decode([]byte(rsaPrivatePem))

	var err error
	rsaPrivate, err = x509.ParsePKCS1PrivateKey(rsaPrivateKeyPem.Bytes)
	if err != nil {
		panic(err)
	}

	rsaPublicKeyPem, _ := pem.Decode([]byte(rsaPublicPem))

	rsaPublic, err = x509.ParsePKCS1PublicKey(rsaPublicKeyPem.Bytes)
	if err != nil {
		panic(err)
	}

	ed25519PrivateKeyPem, _ := pem.Decode([]byte(ed25519PrivatePem))

	ed25519Private, err = x509.ParsePKCS8PrivateKey(ed25519PrivateKeyPem.Bytes)
	if err != nil {
		panic(err)
	}

	ed25519PublicKeyPem, _ := pem.Decode([]byte(ed25519PublicPem))

	ed25519Public, err = x509.ParsePKIXPublicKey(ed25519PublicKeyPem.Bytes)
	if err != nil {
		panic(err)
	}
}

func TestRFC9421_BuildSignatureBase(t *testing.T) {
	for _, data := range []struct {
		Name       string
		Components []string
		Expected   string
	}{
		{
			Name:       "DerivedComponents",
			Components: []string{"@path", "@query", "@target-uri", "@request-target"},
			Expected: `"@path": /foo
"@query": ?param=value&foo=bar&baz=bat%2Dman
"@target-uri": http://origin.host.internal.example/foo?param=value&foo=bar&baz=bat%2Dman
"@request-target": /foo?param=value&foo=bar&baz=bat%2Dman
"@signature-params": abc`,
		},
		{
			Name:       "MultipleValues",
			Components: []string{"host"},
			Expected: `"host": a, b
"@signature-params": abc`,
		},
		{
			Name:       "MissingHeader",
			Components: []string{"date"},
		},
		{
			Name:       "UnsupportedComponent",
			Components: []string{"@host"},
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(http.MethodPost, "http://origin.host.internal.example/foo?param=value&foo=bar&baz=bat%2Dman", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Add("Host", "a ")
			r.Header.Add("Host", " b")

			if base, err := buildSignatureBase(r, "abc", data.Components); err != nil && data.Expected == "" {
				return
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			} else if base != data.Expected {
				t.Fatalf("Wrong base: %s", base)
			}
		})
	}
}

func TestRFC9421_Sign(t *testing.T) {
	t.Parallel()

	for _, data := range []struct {
		Name                             string
		Key                              Key
		Method, URL, Body                string
		Components                       []string
		Alg                              string
		Digest                           func(*http.Request, []byte)
		Now, Expires                     time.Time
		ExpectedInput, ExpectedSignature string
	}{
		{
			Name:              "RSAHappyFlow",
			Key:               Key{ID: "test-key-rsa", PrivateKey: rsaPrivate},
			Method:            http.MethodPost,
			URL:               "http://origin.host.internal.example/foo",
			Body:              `{"hello": "world"}`,
			Components:        []string{"@method", "@authority", "@path", "content-digest", "content-type", "content-length", "forwarded"},
			Alg:               "rsa-v1_5-sha256",
			Digest:            RFC9421DigestSHA512,
			Now:               time.Unix(1618884480, 0),
			Expires:           time.Unix(1618884540, 0),
			ExpectedInput:     `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`,
			ExpectedSignature: "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:",
		},
		{
			Name:              "Ed25519HappyFlow",
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			Method:            http.MethodPost,
			URL:               "http://example.com/foo",
			Body:              `{"hello": "world"}`,
			Components:        []string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			ExpectedInput:     `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:",
		},
		{
			Name:       "EmptyKeyID",
			Key:        Key{PrivateKey: ed25519Private},
			Method:     http.MethodPost,
			URL:        "http://example.com/foo",
			Body:       `{"hello": "world"}`,
			Components: []string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
			Digest:     RFC9421DigestSHA256,
			Now:        time.Unix(1618884473, 0),
		},
		{
			Name:       "InvalidKeyType",
			Key:        Key{ID: "test-key-ed25519", PrivateKey: []byte{}},
			Method:     http.MethodPost,
			URL:        "http://example.com/foo",
			Body:       `{"hello": "world"}`,
			Components: []string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
			Digest:     RFC9421DigestSHA256,
			Now:        time.Unix(1618884473, 0),
		},
		{
			Name:       "SmallKey",
			Key:        Key{ID: "test-key-rsa", PrivateKey: &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: big.NewInt(512)}}},
			Method:     http.MethodPost,
			URL:        "http://origin.host.internal.example/foo",
			Body:       `{"hello": "world"}`,
			Components: []string{"@method", "@authority", "@path", "content-digest", "content-type", "content-length", "forwarded"},
			Alg:        "rsa-v1_5-sha256",
			Digest:     RFC9421DigestSHA512,
			Now:        time.Unix(1618884480, 0),
			Expires:    time.Unix(1618884540, 0),
		},
		{
			Name:       "InvalidComponent",
			Key:        Key{ID: "test-key-rsa", PrivateKey: rsaPrivate},
			Method:     http.MethodPost,
			URL:        "http://origin.host.internal.example/foo",
			Body:       `{"hello": "world"}`,
			Components: []string{"@method", "@authority", "@path", "content-digest", "content-type", "content-length", "forwarded", "@date"},
			Alg:        "rsa-v1_5-sha256",
			Digest:     RFC9421DigestSHA512,
			Now:        time.Unix(1618884480, 0),
			Expires:    time.Unix(1618884540, 0),
		},
		{
			Name:              "PostWithQuery",
			Method:            http.MethodPost,
			URL:               "http://example.com/foo?a=b",
			Body:              `{"hello": "world"}`,
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			ExpectedInput:     `sig1=("@method" "@target-uri" "@query" "content-type" "content-digest");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:r9CLTnSBs9DptOTMZIIxYR+0WyR3dkRfYPdukkFcUkBILhqIldLAf0AxWXMo7fS9pF9iOc2FWhXrupjvTaS9Aw==:",
		},
		{
			Name:              "PostWithoutQuery",
			Method:            http.MethodPost,
			URL:               "http://example.com/foo",
			Body:              `{"hello": "world"}`,
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			ExpectedInput:     `sig1=("@method" "@target-uri" "content-type" "content-digest");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:hxRBa6+EN30jMm8FHHCdksP89LHnQRM47LqdewNSNwQOPDT1NWCatNlL5ew3iMHoTD3iDianApepcmTGxXFxDg==:",
		},
		{
			Name:              "GetWithQuery",
			Method:            http.MethodGet,
			URL:               "http://example.com/foo?a=b",
			Body:              `{"hello": "world"}`,
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			ExpectedInput:     `sig1=("@method" "@target-uri" "@query");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:hUN/1cXurzP2kE30k5hhl46XUnFYiTWhbabGChyzUQV2aWSobjHCtY+qLyru3UJC/p04i6WQYsXNtlYT+T89AQ==:",
		},
		{
			Name:              "GetWithoutQuery",
			Method:            http.MethodGet,
			URL:               "http://example.com/foo",
			Body:              `{"hello": "world"}`,
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			ExpectedInput:     `sig1=("@method" "@target-uri");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:hxZ3eAZwW0a0OuyAWd+U+k8WBhzESrnmP+9HZoSNX46JFsc4bJ0Nib5OXq4tINosJI4ACR8J0Ogi+5h4F5YkDA==:",
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(data.Method, data.URL, strings.NewReader(data.Body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")

			if err := SignRFC9421(
				r,
				[]byte(data.Body),
				data.Key,
				data.Now,
				data.Expires,
				data.Digest,
				data.Alg,
				data.Components,
			); err != nil && data.ExpectedInput == "" {
				return
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if s := r.Header.Get("Signature-Input"); s != data.ExpectedInput {
				t.Fatalf("Wrong signature input: %s", s)
			}

			if s := r.Header.Get("Signature"); s != data.ExpectedSignature {
				t.Fatalf("Wrong signature: %s", s)
			}
		})
	}
}

func TestRFC9421_VerifyHappyFlow(t *testing.T) {
	t.Parallel()

	for _, data := range []struct {
		Name                            string
		URL                             string
		Key                             any
		Now                             time.Time
		ContentDigest, Input, Signature string
	}{
		{
			Name:          "RSA",
			URL:           "http://origin.host.internal.example/foo",
			Key:           rsaPublic,
			Now:           time.Unix(1618884539, 0),
			ContentDigest: "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:",
			Input:         `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`,
			Signature:     "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:",
		},
		{
			Name:          "Ed25519",
			URL:           "http://example.com/foo",
			Key:           ed25519Public,
			Now:           time.Unix(1618884474, 0),
			ContentDigest: "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:",
			Input:         `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`,
			Signature:     "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:",
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(http.MethodPost, data.URL, strings.NewReader(`{"hello": "world"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")
			r.Header.Set("Content-Digest", data.ContentDigest)
			r.Header.Set("Signature", data.Signature)

			sig, err := rfc9421Extract(r, data.Input, []byte(`{"hello": "world"}`), r.URL.Host, data.Now, time.Minute, nil)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			if err := sig.Verify(data.Key); err != nil {
				t.Fatalf("Failed to verify: %v", err)
			}
		})
	}
}

func TestRFC9421_VerifyFailure(t *testing.T) {
	t.Parallel()

	for _, data := range []struct {
		Name   string
		Key    any
		Mutate func(r *http.Request)
	}{
		{
			Name: "TwoSignatures",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Add("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")
			},
		},
		{
			Name: "TwoContentDigest",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Add("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
			},
		},
		{
			Name: "InvalidBase64",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw=:")
			},
		},
		{
			Name: "CreatedNotNumber",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=16188a84473;keyid="test-key-ed25519"`)
			},
		},
		{
			Name: "Expired",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";expires=1618884474`)
			},
		},
		{
			Name: "ExpiresNotNumber",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";expires=23834a95214`)
			},
		},
		{
			Name: "TwoAlg",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="ed25519";alg="ed25519"`)
			},
		},
		{
			Name: "InvalidAlg",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`)
			},
		},
		{
			Name: "AlgNoQuotes",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg=ed25519"`)
			},
		},
		{
			Name: "InvalidHost",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.URL.Host = "example.org"
			},
		},
		{
			Name: "InvalidSignatureInput",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`)
			},
		},
		{
			Name: "InvalidSignature",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", "=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")
			},
		},
		{
			Name: "LabelMismatch",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig2=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`)
			},
		},
		{
			Name: "DuplicateComponent",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "date" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`)
			},
		},
		{
			Name: "MissingRequiredComponent",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`)
			},
		},
		{
			Name: "TwoKeyIDs",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";keyid="test-key-ed25519"`)
			},
		},
		{
			Name: "KeyIDNoQuotes",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid=test-key-ed25519"`)
			},
		},
		{
			Name: "TwoCreated",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;created=1618884473;keyid="test-key-ed25519"`)
			},
		},
		{
			Name: "TwoExpires",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";expires=2383495214;expires=2383495214`)
			},
		},
		{
			Name: "AddedTag",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";tag="a"`)
			},
		},
		{
			Name: "InvalidParameter",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";invalid="a"`)
			},
		},
		{
			Name: "NoKeyId",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473`)
			},
		},
		{
			Name: "NoCreated",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");keyid="test-key-ed25519"`)
			},
		},
		{
			Name: "NoContentDigest",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Del("Content-Digest")
			},
		},
		{
			Name: "EmptyContentDigest",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "")
			},
		},
		{
			Name: "InvalidContentDigest",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
			},
		},
		{
			Name: "InvalidContentDigestBase64",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE:")
			},
		},
		{
			Name: "InvalidContentDigestSha256Size",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-256=:H0D8ktokFpR1CXnubPWC8tXX0o4YM13gWrxU0FYOD1MChgxlK/CNVgJSql50IQVG82n7u86MEs/HlXsmUv6adQ==:")
			},
		},
		{
			Name: "InvalidContentDigestSha256Mismatch",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-256=:ypeBEsobvcr6wjGzmiPcTaeG7/gUfE5yuYB3ha/uSLs=:")
			},
		},
		{
			Name: "InvalidContentDigestSha512Size",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-512=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
			},
		},
		{
			Name: "InvalidContentDigestSha512Mismatch",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-512=:H0D8ktokFpR1CXnubPWC8tXX0o4YM13gWrxU0FYOD1MChgxlK/CNVgJSql50IQVG82n7u86MEs/HlXsmUv6adQ==:")
			},
		},
		{
			Name: "InvalidContentDigestInvalidType",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-1=:hvfkN/qlp/zhXR3cuerq6jd2Z7g=:")
			},
		},
		{
			Name: "InvalidComponent",
			Key:  ed25519Public,
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "@content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`)
			},
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")
			r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
			r.Header.Set("Signature-Input", `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`)
			r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

			data.Mutate(r)

			sig, err := rfc9421Extract(r, r.Header.Get("Signature-Input"), []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884475, 0), time.Hour*24*365*10, []string{"@method", "@path", "date"})
			if err != nil {
				return
			}

			if err := sig.Verify(data.Key); err == nil {
				t.Fatal("Expected error")
			}
		})
	}
}

func TestRFC9421_VerifySignatureAge(t *testing.T) {
	t.Parallel()

	for _, data := range []struct {
		Name                            string
		URL                             string
		Key                             any
		Now                             time.Time
		ContentDigest, Input, Signature string
	}{
		{
			Name:          "Ed25519",
			URL:           "http://example.com/foo",
			Key:           ed25519Public,
			Now:           time.Unix(1618884473, 0).Add(time.Minute + time.Second),
			ContentDigest: "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:",
			Input:         `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`,
			Signature:     "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:",
		},
		{
			Name:          "Ed25519",
			URL:           "http://example.com/foo",
			Key:           ed25519Public,
			Now:           time.Unix(1618884473, 0).Add(-time.Minute - time.Second),
			ContentDigest: "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:",
			Input:         `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`,
			Signature:     "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:",
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(http.MethodPost, data.URL, strings.NewReader(`{"hello": "world"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")
			r.Header.Set("Content-Digest", data.ContentDigest)
			r.Header.Set("Signature", data.Signature)

			if _, err := rfc9421Extract(r, data.Input, []byte(`{"hello": "world"}`), r.URL.Host, data.Now, time.Minute, nil); err == nil {
				t.Fatal("Expected error")
			}
		})
	}
}

// https://github.com/C2SP/C2SP/blob/3bc97b2329fee167f7ff39efbbbc316c84876105/httpsig-pq.md?plain=1#L253
func TestRFC9421_MLDSA44(t *testing.T) {
	t.Parallel()

	rawPub, err := base64.StdEncoding.DecodeString("guWbbXQe/Y1a83/qIeU8MjIrRseICPuOyWzoGXk27Fia2lpk2uPxbEfh/iEmvY1z/LTtxSBFZS8zbgV04qa038ZokwE+QoBcISX0H4F9hEY3Oxg8SRtzrek0E+mBOc6R4ilBje5vodImgSMTMSXKuDz2PWqvrLY20AfLmMkLXZqVERYcW+S7iKyTP53cvtenYwealt3MGcZxLKRAMnCGBY3pRfj1xn6czY6OuMYNuJaW1rAeL2BeVpNA+XBA+HUHS1yTYOGw93ddFe9lw0RDex7Pw+tht/n31gggrR4D0kmE8L88Q5c2qWuQtxk9+cmHTm3u1WuG/vCRiubTA3DmY0pK1fO1etarscNRnZ06q5UfTgSQOMqR/PLXtqT9cZiN8GWQvum15uVuxXCubB4r7T3nz2ijyBPQfo+Ywd10QxI8dpGpsBd9SEslLo+esyFDt7+9t4c702PNC0121oI18PriNlKxjxshQljNnrXSZ4eCvhHrY5Le8B/mTwsb/LYPbBvktCecvkRxboTjhyq2anblDt5Hp55gn+0YRWUz7myuDjBD/+a2riHzOXCoLOtGvsfVTLAwLPYj/AIBbdVFrGq9MKldtcgiqTWhG6JWD/dxFrfUT4YIf2yWO9AbhkgkgPynKmV82sLda4L3JaABwSXVAB1vzDmxfQ17orBFxX2pK23G5EmrG8Ix/1O9NJ35LSxkZhUmNs+4zm9GRaChPbw04q//Gc9CPiE3xvv/BrRms5mIiRl7zT8o2mgNs/APM45mJrKClmYRQllKVwnKBj+njZkDv1CG2WeBv01COmic4P4EEraSmKO8I5sJ98gpTiA9jeBuvloLYdAInPyZui01GyQO4P3OETyAVqggBGIvajxWuio7eL+4XZmkX3AMb5XrfWtlfYCKUzhodcv7NM6M4LizEh0KZ8qSHsnVZxqwb5J9hb4oiR5/N7sgxjjtiBsbg/vJ1UmBZZR6hPjnLbLDTZ6PUMjeQNdoYGL0Cv4ESloaEtc4vhW/Aa2roIyoAM/kimrHYztNknxZK3nLQ9PXVp9Np1okp4VPUkOfHvy6Bk2WPQd8vdwrfE3xOPGrkhzdmlcsVEp3rIAW4s4Hem2A99CFSkbPePEEPG4uB3sVl73X+sr8+X7jFkiiy686eV73zQTrqL4OnrYJIkaZr6zLXHccvm1txmftmtS3WH3Ny0hGVjwxnu1femv2++2pZpZr67//WBksxPOG4xk6AA7xrHEAjRFA0b0g6AC3+WuZmaNCNMd8YdibEt/WHKg7uGWYetMf45cWxPxZmNl+jddlf1btTAfYZ+eXla3Tx/pg5fU2EWTXyTOVIRdRwAOpZHG7jFYCxRE2SWFFM2lwjLxzso69uABhM1zraUxo6TfOalw1x7S1NkTowesaJPlTMmr43N5hdbRyDD6qmcCSNNjDICvK/fv3ijSY0lueglt1Jf7O1fi9/s0CNGf8E/pQohdb/pzGhSlJRo73hC/wTmLEjFukAYNdNeJO8Fxge7j6rLHIOu22q8lK0DSsWqXXeFYt5mKmdTOFHN6soJQ7Uk43HgwoMreCn6xJQhl87CZu4SUzr5XztrsOV/YF5u5IYY8cDneoZW+0ldE+/8bvI8gi352YmfL9ZY5TacDexbl0S+hGbR7IPHh6av0N+odj2uBNt4Cjb3Mt3aYwQi2Yh9vD+CaGcz5AO67GAf7pIpS8naBrzQeNjdOYLxu6VcXT4zLO9Gu1Uv+RvkLPaalcAQ==")
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}

	var pub mldsa44.PublicKey
	if err := pub.UnmarshalBinary(rawPub[:]); err != nil {
		t.Fatalf("Failed to parse public key: %v", err)
	}

	r, err := http.NewRequest(http.MethodGet, "https://example.com/foo?param=Value&Pet=dog", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	r.Header.Set("Host", "example.com")
	r.Header.Set("Date", "Mon, 06 Jul 2026 20:00:00 GMT")
	r.Header.Set("Signature", "sig1=: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:")

	sig, err := rfc9421Extract(r, `sig1=("@method" "@target-uri" "host" "date");created=1783368000;keyid="test-key-mldsa44";alg="ml-dsa-44"`, nil, r.URL.Host, time.Unix(1783368000, 0), time.Minute, nil)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if err := sig.Verify(&pub); err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}
}
