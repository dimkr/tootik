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

var (
	DIDKeyRegex        = regexp.MustCompile(`^did:key:(z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)(?:[\/#?].*){0,1}`)
	PortableIDRegex    = regexp.MustCompile(`^ap:\/\/did:key:(z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)((?:[\/#?].*){0,1})`)
	CompatibleURLRegex = regexp.MustCompile(`^https:\/\/[a-z0-9-]+(?:\.[a-z0-9-]+)+\/\.well-known\/apgateway\/did:key:(z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)((?:[\/#?].*){0,1})`)
)

// IsPortable determines whether or not an ActivityPub ID is portable.
func IsPortable(id string) bool {
	return PortableIDRegex.MatchString(id) || CompatibleURLRegex.MatchString(id)
}

// Abs prepends ap:// or https:// to a string to obtain a valid ActivityPub ID.
func Abs(s string) string {
	if DIDKeyRegex.MatchString(s) {
		return "ap://" + s
	}

	return "https://" + s
}

// Canonical returns an ID in canonical form: if portable, it's converted to an ap:// URL.
func Canonical(id string) string {
	if PortableIDRegex.MatchString(id) {
		return id
	}

	if m := CompatibleURLRegex.FindStringSubmatch(id); m != nil {
		return "ap://did:key:" + m[1] + m[2]
	}

	return id
}

// Gateway returns a https:// gateway URL for a portable ActivityPub ID.
func Gateway(gw, id string) string {
	if m := PortableIDRegex.FindStringSubmatch(id); m != nil {
		return fmt.Sprintf("%s/.well-known/apgateway/did:key:%s%s", gw, m[1], m[2])
	}

	if m := CompatibleURLRegex.FindStringSubmatch(id); m != nil {
		return fmt.Sprintf("%s/.well-known/apgateway/did:key:%s%s", gw, m[1], m[2])
	}

	return id
}

// GetOrigin returns the origin of an ActivityPub ID.
func GetOrigin(id string) (string, error) {
	if m := PortableIDRegex.FindStringSubmatch(id); m != nil {
		return "did:key:" + m[1], nil
	}

	if m := CompatibleURLRegex.FindStringSubmatch(id); m != nil {
		return "did:key:" + m[1], nil
	}

	if DIDKeyRegex.MatchString(id) {
		return id, nil
	}

	if u, err := url.Parse(id); err != nil {
		return "", err
	} else {
		return u.Host, nil
	}
}
