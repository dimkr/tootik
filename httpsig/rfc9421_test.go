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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestRFC9421_RSASign(t *testing.T) {
	t.Parallel()

	// 4.3.  Multiple Signatures
	r, err := http.NewRequest(http.MethodPost, "http://origin.host.internal.example/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:56 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")

	if err := SignRFC9421(
		r,
		Key{
			ID:         "test-key-rsa",
			PrivateKey: rsaPrivate,
		},
		time.Unix(1618884480, 0),
		time.Unix(1618884540, 0),
		RFC9421DigestSHA512,
		"rsa-v1_5-sha256",
		[]string{"@method", "@authority", "@path", "content-digest", "content-type", "content-length", "forwarded"},
	); err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if s := r.Header.Get("Content-Digest"); s != "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:" {
		t.Fatalf("Wrong digest: %s", s)
	}

	if s := r.Header.Get("Signature"); s != "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:" {
		t.Fatalf("Wrong signature: %s", s)
	}
}

func TestRFC9421_ED25519Sign(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")

	if err := SignRFC9421(
		r,
		Key{
			ID:         "test-key-ed25519",
			PrivateKey: ed25519Private,
		},
		time.Unix(1618884473, 0),
		time.Time{},
		RFC9421DigestSHA256,
		"",
		[]string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
	); err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if s := r.Header.Get("Content-Digest"); s != "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:" {
		t.Fatalf("Wrong digest: %s", s)
	}

	if s := r.Header.Get("Signature"); s != "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:" {
		t.Fatalf("Wrong signature: %s", s)
	}
}

func TestRFC9421_RSAVerifyHappyFlow(t *testing.T) {
	t.Parallel()

	// 4.3.  Multiple Signatures
	r, err := http.NewRequest(http.MethodPost, "http://origin.host.internal.example/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:56 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")
	r.Header.Set("Content-Digest", "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:")

	sig, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "origin.host.internal.example", time.Unix(1618884539, 0), time.Second, nil)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if err := sig.Verify(rsaPublic); err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}
}

func TestRFC9421_ED25519VerifyHappyFlow(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	sig, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if err := sig.Verify(ed25519Public); err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}
}

func TestRFC9421_RSAVerifyFailure(t *testing.T) {
	for _, data := range []struct {
		Name   string
		Mutate func(*http.Request)
		Now    time.Time
	}{
		{
			Name: "WrongMethod",
			Mutate: func(r *http.Request) {
				r.Method = http.MethodGet
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongAuthority",
			Mutate: func(r *http.Request) {
				r.URL.Host = "a"
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongPath",
			Mutate: func(r *http.Request) {
				r.URL.Path = "/a"
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoContentDigest",
			Mutate: func(r *http.Request) {
				r.Header.Del("Content-Digest")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongContentDigest",
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Digest", "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvjwew==:")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongContentType",
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Type", "a")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongContentLength",
			Mutate: func(r *http.Request) {
				r.Header.Set("Content-Length", "19")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongForwarded",
			Mutate: func(r *http.Request) {
				r.Header.Set("Forwarded", "a")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name:   "Expired",
			Mutate: func(r *http.Request) {},
			Now:    time.Unix(1618884541, 0),
		},
		{
			Name: "NoSeparator",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoComponents",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "EmptyComponents",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=();created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoLeftParenthesis",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1="@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoRightParenthesis",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded";created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoLeftQuotes",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" @authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoRightQuotes",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "DuplicateComponent",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded" "@path");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "MissingRequiredComponent",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoSignature",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", `sig1=`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoLeftColon",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", "sig1=S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NothingBetweenColons",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", "sig1=::")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "NoRightColon",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature", "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==")
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongLabel",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig2=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "DuplicateKeyID",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540;keyid="test-key-rsa"`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "DuplicateCreated",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540;created=1618884480`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "DuplicateExpires",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540;expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(http.MethodPost, "http://origin.host.internal.example/foo", strings.NewReader(`{"hello": "world"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:56 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")
			r.Header.Set("Content-Digest", "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:")
			r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`)
			r.Header.Set("Signature", "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:")

			data.Mutate(r)

			if sig, err := rfc9421Extract(
				r,
				r.Header.Get("Signature-Input"),
				[]byte(`{"hello": "world"}`),
				"origin.host.internal.example",
				data.Now,
				time.Second,
				[]string{"@method", "@authority"},
			); err != nil {
				return
			} else if err := sig.Verify(rsaPublic); err == nil {
				t.Fatal("Verification was supposed to fail")
			}
		})
	}
}

func TestRFC9421_Query(t *testing.T) {
	t.Parallel()

	r, err := http.NewRequest(http.MethodPost, "http://origin.host.internal.example/foo?param=value&foo=bar&baz=bat%2Dman", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	const expected = `"@path": /foo
"@query": ?param=value&foo=bar&baz=bat%2Dman
"@signature-params": abc`

	if base, err := buildSignatureBase(r, "abc", []string{"@path", "@query"}); err != nil {
		t.Fatalf("Failed to build base: %v", err)
	} else if base != expected {
		t.Fatalf("Wrong base: %s", base)
	}
}
