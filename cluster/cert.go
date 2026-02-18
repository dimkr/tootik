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

package cluster

import "crypto/tls"

const (
	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=alice" -out cert.pem -keyout key.pem -days 3650
	*/
	aliceCert = `-----BEGIN CERTIFICATE-----
MIIBdDCCARugAwIBAgIUJuOAcrlKylHyzCtvoAcXhRxrLe8wCgYIKoZIzj0EAwIw
EDEOMAwGA1UEAwwFYWxpY2UwHhcNMjQxMjE3MDcwOTU3WhcNMzQxMjE1MDcwOTU3
WjAQMQ4wDAYDVQQDDAVhbGljZTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABAN1
aOlRXxwYPaoMM7hT35SO41MhbI905fRLcoD8ACZSsTk7Btl85wpRwLbUiDubPpD9
YpznOMtLp/+PsOKhWJyjUzBRMB0GA1UdDgQWBBR+sP/lgsWpZNiO6MbPc9bnavIp
XDAfBgNVHSMEGDAWgBR+sP/lgsWpZNiO6MbPc9bnavIpXDAPBgNVHRMBAf8EBTAD
AQH/MAoGCCqGSM49BAMCA0cAMEQCIA3VpVxWjNXZPT74SfKama9HZ5cmTMWH2i30
Js+ELyWFAiAg5YcOWz0rAYF3vo3qnSqm+7jm39K4od8R0BMPg70zHg==
-----END CERTIFICATE-----`

	aliceKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgqqMaa2ACcF4UD2Cf
dfpDNbVZQ+mxG2+tFz8vEy4Y6RyhRANCAAQDdWjpUV8cGD2qDDO4U9+UjuNTIWyP
dOX0S3KA/AAmUrE5OwbZfOcKUcC21Ig7mz6Q/WKc5zjLS6f/j7DioVic
-----END PRIVATE KEY-----`

	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=bob" -out cert.pem -keyout key.pem -days 3650
	*/
	bobCert = `-----BEGIN CERTIFICATE-----
MIIBcDCCARegAwIBAgIUHHc5FBqE6nlGI33AdS0Om1lx2jMwCgYIKoZIzj0EAwIw
DjEMMAoGA1UEAwwDYm9iMB4XDTI0MTIxNzA3MTA1MFoXDTM0MTIxNTA3MTA1MFow
DjEMMAoGA1UEAwwDYm9iMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDEyoUvPQ
jEkbqT7LEo3QEnUbt8Y6HETTW6FxPQNlwCYSQ6VAyQWw1Y1XzWbXCC9iagvy7U5Y
NoC4MvZVUwAGtqNTMFEwHQYDVR0OBBYEFJML/a9F7xhc/hjoT6SJu/y/4+sCMB8G
A1UdIwQYMBaAFJML/a9F7xhc/hjoT6SJu/y/4+sCMA8GA1UdEwEB/wQFMAMBAf8w
CgYIKoZIzj0EAwIDRwAwRAIgaYNUFZcOVM8e5of+CdnMcIYJ84sHL5+pVqUwmxfg
zfsCIBK1ngZLPUA0hWg3H1KQ20cztCsZe+pGXnP6WBZkadwT
-----END CERTIFICATE-----`

	bobKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgLQ/sf07/vkgCMkGq
Sb/WvHBG/5kqDqmxBL3SVNtSQAWhRANCAAQMTKhS89CMSRupPssSjdASdRu3xjoc
RNNboXE9A2XAJhJDpUDJBbDVjVfNZtcIL2JqC/LtTlg2gLgy9lVTAAa2
-----END PRIVATE KEY-----`

	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=carol" -out cert.pem -keyout key.pem -days 3650
	*/
	carolCert = `-----BEGIN CERTIFICATE-----
MIIBdTCCARugAwIBAgIUXKYrdScbXT6lcO6CCuX8OkJCGZ4wCgYIKoZIzj0EAwIw
EDEOMAwGA1UEAwwFY2Fyb2wwHhcNMjQxMjE3MDcxMTI4WhcNMzQxMjE1MDcxMTI4
WjAQMQ4wDAYDVQQDDAVjYXJvbDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABHL+
6+MbIEb4LcuFSYGhgH1BoI0tDfnXSjuEd9FHOJsfZHPkpha47ogth7webGcRNF83
WRjx9dP8UEosxr0cbpKjUzBRMB0GA1UdDgQWBBS7B3n2ipt/kHz2rxsPEcj0Y8dT
OzAfBgNVHSMEGDAWgBS7B3n2ipt/kHz2rxsPEcj0Y8dTOzAPBgNVHRMBAf8EBTAD
AQH/MAoGCCqGSM49BAMCA0gAMEUCIQC+xOm7tdu0OoUaNOxq4KMJ8Jjdh7Unnbw6
H0DXeYT/ogIgUdv59y9qUKagjNNOUTs1pMgsBz1Mybb8sOTWK5UvwFA=
-----END CERTIFICATE-----`

	carolKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgdMw3ufTLDC79nyOQ
fQrvoiJJO5IM6yK73Wu9mlPh9YShRANCAARy/uvjGyBG+C3LhUmBoYB9QaCNLQ35
10o7hHfRRzibH2Rz5KYWuO6ILYe8HmxnETRfN1kY8fXT/FBKLMa9HG6S
-----END PRIVATE KEY-----`

	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=dave" -out cert.pem -keyout key.pem -days 3650
	*/
	daveCert = `-----BEGIN CERTIFICATE-----
MIIBczCCARmgAwIBAgIUVyeE7+4UEFWn0QWRdMnu4wPsshQwCgYIKoZIzj0EAwIw
DzENMAsGA1UEAwwEZGF2ZTAeFw0yNjAyMTgxOTI5MThaFw0zNjAyMTYxOTI5MTha
MA8xDTALBgNVBAMMBGRhdmUwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATvD3BA
jILwBoa7FFuPPuR73bFHPY+6Fq+/Pg7MtmrNo5b7rnpqmPGvA6wTwWFYULRYt+v+
nQZzAfKAEpSWGvOGo1MwUTAdBgNVHQ4EFgQU+9aBjSL9X5I/G5FX8+7UdyLeaCAw
HwYDVR0jBBgwFoAU+9aBjSL9X5I/G5FX8+7UdyLeaCAwDwYDVR0TAQH/BAUwAwEB
/zAKBggqhkjOPQQDAgNIADBFAiAeA8ujvl0llQZDu7ZDWDQb+Mufi5qG7bCCKfsI
PhcuvAIhANSwASOh8TfjBQHjZGWL2fbiRyJ1XENF3/ue9e2rltau
-----END CERTIFICATE-----`

	daveKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgHaHQSZiK+NDCiI/7
r0QX1AP8sHa8RZu+hQcw42x1BUOhRANCAATvD3BAjILwBoa7FFuPPuR73bFHPY+6
Fq+/Pg7MtmrNo5b7rnpqmPGvA6wTwWFYULRYt+v+nQZzAfKAEpSWGvOG
-----END PRIVATE KEY-----`
)

var aliceKeypair, bobKeypair, carolKeypair, daveKeypair tls.Certificate

func init() {
	var err error
	aliceKeypair, err = tls.X509KeyPair([]byte(aliceCert), []byte(aliceKey))
	if err != nil {
		panic(err)
	}

	bobKeypair, err = tls.X509KeyPair([]byte(bobCert), []byte(bobKey))
	if err != nil {
		panic(err)
	}

	carolKeypair, err = tls.X509KeyPair([]byte(carolCert), []byte(carolKey))
	if err != nil {
		panic(err)
	}

	daveKeypair, err = tls.X509KeyPair([]byte(daveCert), []byte(daveKey))
	if err != nil {
		panic(err)
	}
}
