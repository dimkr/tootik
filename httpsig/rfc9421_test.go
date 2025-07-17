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
		{
			Name: "UnspecifiedAlg",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";expires=1618884540`)
			},
			Now: time.Unix(1618884481, 0),
		},
		{
			Name: "WrongAlg",
			Mutate: func(r *http.Request) {
				r.Header.Set("Signature-Input", `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="ed25519";expires=1618884540`)
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
		Method, URL                      string
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
			Components: []string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
			Digest:     RFC9421DigestSHA256,
			Now:        time.Unix(1618884473, 0),
		},
		{
			Name:       "InvalidKeyType",
			Key:        Key{ID: "test-key-ed25519", PrivateKey: []byte{}},
			Method:     http.MethodPost,
			URL:        "http://example.com/foo",
			Components: []string{"date", "@method", "@path", "@authority", "content-type", "content-length"},
			Digest:     RFC9421DigestSHA256,
			Now:        time.Unix(1618884473, 0),
		},
		{
			Name:              "PostWithQuery",
			Method:            http.MethodPost,
			URL:               "http://example.com/foo?a=b",
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
			Digest:            RFC9421DigestSHA256,
			Now:               time.Unix(1618884473, 0),
			Key:               Key{ID: "test-key-ed25519", PrivateKey: ed25519Private},
			ExpectedInput:     `sig1=("@method" "@target-uri");created=1618884473;keyid="test-key-ed25519"`,
			ExpectedSignature: "sig1=:hxZ3eAZwW0a0OuyAWd+U+k8WBhzESrnmP+9HZoSNX46JFsc4bJ0Nib5OXq4tINosJI4ACR8J0Ogi+5h4F5YkDA==:",
		},
	} {
		t.Run(data.Name, func(tt *testing.T) {
			tt.Parallel()

			r, err := http.NewRequest(data.Method, data.URL, strings.NewReader(`{"hello": "world"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Length", "18")
			r.Header.Set("Forwarded", "for=192.0.2.123;host=example.com;proto=https")

			if err := SignRFC9421(
				r,
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

func TestRFC9421_Verify(t *testing.T) {
	t.Parallel()

	for _, data := range []struct {
		Name                            string
		URL                             string
		Key                             any
		Now                             time.Time
		ContentDigest, Input, Signature string
	}{
		{
			Name:          "RSAHappyFlow",
			URL:           "http://origin.host.internal.example/foo",
			Key:           rsaPublic,
			Now:           time.Unix(1618884539, 0),
			ContentDigest: "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:",
			Input:         `sig1=("@method" "@authority" "@path" "content-digest" "content-type" "content-length" "forwarded");created=1618884480;keyid="test-key-rsa";alg="rsa-v1_5-sha256";expires=1618884540`,
			Signature:     "sig1=:S6ZzPXSdAMOPjN/6KXfXWNO/f7V6cHm7BXYUh3YD/fRad4BCaRZxP+JH+8XY1I6+8Cy+CM5g92iHgxtRPz+MjniOaYmdkDcnL9cCpXJleXsOckpURl49GwiyUpZ10KHgOEe11sx3G2gxI8S0jnxQB+Pu68U9vVcasqOWAEObtNKKZd8tSFu7LB5YAv0RAGhB8tmpv7sFnIm9y+7X5kXQfi8NMaZaA8i2ZHwpBdg7a6CMfwnnrtflzvZdXAsD3LH2TwevU+/PBPv0B6NMNk93wUs/vfJvye+YuI87HU38lZHowtznbLVdp770I6VHR6WfgS9ddzirrswsE1w5o0LV/g==:",
		},
		{
			Name:          "Ed25519HappyFlow",
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

func TestRFC9421_VerifyTwoSignatures(t *testing.T) {
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
	r.Header.Add("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidBase64(t *testing.T) {
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
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw=:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyNoKeyIDQuotes(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyCreatedNotNumber(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=16188a84473;keyid="test-key-ed25519"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyExpiresNotNumber(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";expires=16188a84540`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyTwoAlg(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="ed5519";alg="ed5519"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyAlgQuotes(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="ed5519`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidAlg(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519";alg="rc4"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyUnknownParameter(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyida="test-key-ed25519"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyNoKeyID(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyNoCreated(t *testing.T) {
	t.Parallel()

	// B.2.6.  Signing a Request Using ed25519
	r, err := http.NewRequest(http.MethodPost, "http://example.com/foo", strings.NewReader(`{"hello": "world"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	sigInput := `sig1=("date" "@method" "@path" "@authority" "content-type" "content-length");keyid="test-key-ed25519"`

	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "18")
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyTwoContentDigest(t *testing.T) {
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
	r.Header.Add("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyEmptyContentDigest(t *testing.T) {
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
	r.Header.Set("Content-Digest", "")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigest(t *testing.T) {
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
	r.Header.Set("Content-Digest", "X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestBase64(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestAlgo(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha1=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestSha256Size(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha-256=:H0D8ktokFpR1CXnubPWC8tXX0o4YM13gWrxU0FYOD1MChgxlK/CNVgJSql50IQVG82n7u86MEs/HlXsmUv6adQ==:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestSha512Size(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha-512=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestSha256Mismatch(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha-256=:ypeBEsobvcr6wjGzmiPcTaeG7/gUfE5yuYB3ha/uSLs=:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestRFC9421_VerifyInvalidContentDigestSha512Mismatch(t *testing.T) {
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
	r.Header.Set("Content-Digest", "sha-512=:H0D8ktokFpR1CXnubPWC8tXX0o4YM13gWrxU0FYOD1MChgxlK/CNVgJSql50IQVG82n7u86MEs/HlXsmUv6adQ==:")
	r.Header.Set("Signature-Input", sigInput)
	r.Header.Set("Signature", "sig1=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:")

	if _, err := rfc9421Extract(r, sigInput, []byte(`{"hello": "world"}`), "example.com", time.Unix(1618884474, 0), time.Second, nil); err == nil {
		t.Fatal("Expected error")
	}
}
