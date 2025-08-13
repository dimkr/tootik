/*
Copyright 2025 Dima Krasner

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

package ap

import (
	"fmt"
	"net/url"
	"regexp"
)

var DIDKeyRegex = regexp.MustCompile(`(?:^ap:\/\/did:key:|.well-known\/apgateway\/did:key:|^did:key:|^)(z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)([\/#?].*){0,1}`)

func IsPortable(id string) bool {
	return DIDKeyRegex.MatchString(id)
}

func Canonicalize(id string) string {
	if m := DIDKeyRegex.FindStringSubmatch(id); m != nil {
		return "ap://did:key:" + m[1] + m[2]
	}

	return id
}

func Gateway(domain, id string) string {
	if m := DIDKeyRegex.FindStringSubmatch(id); m != nil {
		return fmt.Sprintf("https://%s/.well-known/apgateway/did:key:%s%s", domain, m[1], m[2])
	}

	return id
}

// GetOrigin returns the origin of an object, based on its ID.
func GetOrigin(id string) (string, error) {
	if m := DIDKeyRegex.FindStringSubmatch(id); m != nil {
		return "did:key:" + m[1], nil
	}

	if u, err := url.Parse(id); err != nil {
		return "", err
	} else {
		return u.Host, nil
	}
}
