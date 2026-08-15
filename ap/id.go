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

package ap

import (
	"fmt"
	"net/url"
	"regexp"
)

const (
	ed25519PubBase58 = `z6Mk[a-km-zA-HJ-NP-Z1-9]{44}`
	ed25519PubBase64 = `u7Q[A-Za-z0-9_-]{44}`

	mldsa44PubBase58 = `z4sd[a-km-zA-HJ-NP-Z1-9]{1000}[a-km-zA-HJ-NP-Z1-9]{792}`
	mldsa44PubBase64 = `ukC[A-Za-z0-9_-]{1000}[A-Za-z0-9_-]{750}`

	// PortableActorPubPattern matches public keys in portable actor did:key DIDs.
	PortableActorPubPattern = ed25519PubBase58
)

var (
	// KeyRegex matches any Multibase-encoded public key.
	KeyRegex = regexp.MustCompile(`\b(` + PortableActorPubPattern + `|` + ed25519PubBase64 + `|` + mldsa44PubBase58 + mldsa44PubBase64 + `)(?:[\/#?]|$)`)

	// apURLRegex matches an ap:// URL.
	apURLRegex = regexp.MustCompile(`^ap:\/\/did:key:(` + PortableActorPubPattern + `)([\/#?].*|$)`)

	// GatewayURLRegex matches an https:// gateway URL.
	GatewayURLRegex = regexp.MustCompile(`^https:\/\/[a-z0-9-]+(?:\.[a-z0-9-]+)+\/\.well-known\/apgateway\/did:key:(` + PortableActorPubPattern + `)([\/#?].*|$)`)
)

// IsPortable determines whether or not an ActivityPub ID is portable.
func IsPortable(id string) bool {
	return apURLRegex.MatchString(id) || GatewayURLRegex.MatchString(id)
}

// Canonical returns an ID in canonical form: if portable, it's converted to an ap:// URL.
func Canonical(id string) string {
	if apURLRegex.MatchString(id) {
		return id
	}

	if m := GatewayURLRegex.FindStringSubmatch(id); m != nil {
		return "ap://did:key:" + m[1] + m[2]
	}

	return id
}

// Gateway returns a https:// gateway URL for a portable ActivityPub ID.
func Gateway(gw, id string) string {
	if m := apURLRegex.FindStringSubmatch(id); m != nil {
		return fmt.Sprintf("%s/.well-known/apgateway/did:key:%s%s", gw, m[1], m[2])
	}

	if m := GatewayURLRegex.FindStringSubmatch(id); m != nil {
		return fmt.Sprintf("%s/.well-known/apgateway/did:key:%s%s", gw, m[1], m[2])
	}

	return id
}

// Origins returns the origin and the host of an ActivityPub ID.
func Origins(id string) (string, string, error) {
	u, err := url.Parse(id)
	if err != nil {
		return "", "", err
	}

	if m := apURLRegex.FindStringSubmatch(id); m != nil {
		return "did:key:" + m[1], u.Host, nil
	}

	if m := GatewayURLRegex.FindStringSubmatch(id); m != nil {
		return "did:key:" + m[1], u.Host, nil
	}

	return u.Host, u.Host, nil
}

// Origin returns the origin of an ActivityPub ID.
func Origin(id string) (string, error) {
	origin, _, err := Origins(id)
	return origin, err
}
