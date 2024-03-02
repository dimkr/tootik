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

package data

import (
	"crypto/x509"
	"encoding/pem"
)

// ParsePrivateKey parses private keys.
func ParsePrivateKey(privateKeyPemString string) (any, error) {
	privateKeyPem, _ := pem.Decode([]byte(privateKeyPemString))

	privateKey, err := x509.ParsePKCS8PrivateKey(privateKeyPem.Bytes)
	if err != nil {
		// fallback for openssl<3.0.0
		privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyPem.Bytes)
		if err != nil {
			return nil, err
		}
	}

	return privateKey, nil
}
