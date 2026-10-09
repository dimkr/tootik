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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	ed25519PubBase58 = `z6Mk[a-km-zA-HJ-NP-Z1-9]{44}`
	ed25519PubBase64 = `u7Q[A-Za-z0-9_-]{44}`

	mldsa44PubBase58 = `z4sd[a-km-zA-HJ-NP-Z1-9]{1000}[a-km-zA-HJ-NP-Z1-9]{792}`

	// MLDSA44PubBase64 matches a base64url-encoded ML-DSA-44 public key.
	MLDSA44PubBase64 = `ukC[A-Za-z0-9_-]{1000}[A-Za-z0-9_-]{750}`

	// PortableActorPubPattern matches public keys in portable actor did:key DIDs.
	PortableActorPubPattern = ed25519PubBase58 + `|` + MLDSA44PubBase64

	gatewaySegment = "/.well-known/apgateway/did:"

	gatewayParam = "@gateway"
)

// IDKind is the kind of an ActivityPub ID.
type IDKind int

const (
	// URLID is a non-portable ID, usually a https:// URL.
	URLID IDKind = iota

	// GatewayID is a FEP-ef61 compatible ID: a https:// gateway URL.
	GatewayID

	// PortableID is a FEP-ef61 canonical ID: an ap:// or ap+ef61:// URI.
	PortableID
)

// ID is a parsed ActivityPub ID.
type ID struct {
	Kind   IDKind
	Scheme string

	// Origin is the host of a [URLID] and the did:key DID of a portable ID.
	Origin string

	// Host is the host of a [URLID] or a [GatewayID], or the host of the first gateway of a [PortableID].
	Host string

	// Canonical is the ID in canonical form.
	Canonical string

	// Gateways are the gateway of a [GatewayID] or the decoded @gateway location hints of a [PortableID].
	Gateways []string

	path    string
	query   string
	hasUser bool
}

var (
	// KeyRegex matches any Multibase-encoded public key.
	KeyRegex = regexp.MustCompile(`\b(` + PortableActorPubPattern + `|` + ed25519PubBase64 + `|` + mldsa44PubBase58 + `)(?:[\/#?]|$)`)

	// portableRegex matches the scheme and authority of an ap:// or ap+ef61:// URI.
	portableRegex = regexp.MustCompile(`^(?:ap|ap\+ef61):\/\/(did(?::|%3[Aa])key(?::|%3[Aa]))(` + PortableActorPubPattern + `)([\/?#].*)?$`)

	// GatewayURLRegex matches an https:// gateway URL.
	GatewayURLRegex = regexp.MustCompile(`^https:\/\/[a-z0-9-]+(?:\.[a-z0-9-]+)+\/\.well-known\/apgateway\/did:key:(` + PortableActorPubPattern + `)([\/#?].*|$)`)

	// ErrInvalidID is returned by [ParseID] and [ValidateID] when an ID is malformed or ambiguous.
	ErrInvalidID = errors.New("invalid ID")
)

func splitFragment(s string) (string, string) {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		return s[:i], s[i:]
	}

	return s, ""
}

func parseGateways(rawQuery string) ([]string, error) {
	if rawQuery == "" {
		return nil, nil
	}

	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidID, err)
	}

	var gateways []string
	for key, values := range query {
		if key != gatewayParam {
			return nil, fmt.Errorf("%w: unsupported query parameter %s", ErrInvalidID, key)
		}

		gateways = values
	}

	for i, gw := range gateways {
		u, err := url.Parse(gw)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid gateway %s: %w", ErrInvalidID, gw, err)
		}

		if u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("%w: invalid gateway %s", ErrInvalidID, gw)
		}

		gateways[i] = "https://" + u.Host
	}

	return gateways, nil
}

func parsePortable(id string, m []string) (ID, error) {
	if strings.Contains(m[1], ":") && strings.Contains(m[1], "%") {
		return ID{}, fmt.Errorf("%w: partially encoded authority in %s", ErrInvalidID, id)
	}

	rest := m[3]
	if !strings.HasPrefix(rest, "/") {
		return ID{}, fmt.Errorf("%w: no path in %s", ErrInvalidID, id)
	}

	head, fragment := splitFragment(rest)

	u, err := url.ParseRequestURI(head)
	if err != nil {
		return ID{}, fmt.Errorf("%w: %w", ErrInvalidID, err)
	}

	gateways, err := parseGateways(u.RawQuery)
	if err != nil {
		return ID{}, err
	}

	path, _, _ := strings.Cut(head, "?")

	parsed := ID{
		Kind:      PortableID,
		Origin:    "did:key:" + m[2],
		Canonical: "ap://did:key:" + m[2] + path + fragment,
		Gateways:  gateways,
		Scheme:    "ap",
		path:      path,
	}

	if len(gateways) > 0 {
		parsed.Host = gateways[0][len("https://"):]
	}

	return parsed, nil
}

// ParseID parses an ActivityPub ID.
func ParseID(id string) (ID, error) {
	if m := portableRegex.FindStringSubmatch(id); m != nil {
		return parsePortable(id, m)
	}

	u, err := url.Parse(id)
	if err != nil {
		return ID{}, err
	}

	if u.Scheme == "ap" || u.Scheme == "ap+ef61" {
		return ID{}, fmt.Errorf("%w: unsupported portable ID %s", ErrInvalidID, id)
	}

	parsed := ID{
		Kind:      URLID,
		Origin:    u.Host,
		Host:      u.Host,
		Canonical: id,
		Scheme:    u.Scheme,
		path:      u.Path,
		query:     u.RawQuery,
		hasUser:   u.User != nil,
	}

	if strings.Count(id, gatewaySegment) > 1 {
		return ID{}, fmt.Errorf("%w: ambiguous gateway URL %s", ErrInvalidID, id)
	}

	if m := GatewayURLRegex.FindStringSubmatch(id); m != nil {
		head, fragment := splitFragment(m[2])
		path, _, _ := strings.Cut(head, "?")

		parsed.Kind = GatewayID
		parsed.Origin = "did:key:" + m[1]
		parsed.Canonical = "ap://did:key:" + m[1] + path + fragment
		parsed.Gateways = []string{"https://" + u.Host}
	}

	return parsed, nil
}

// ValidateID parses an ActivityPub ID and determines whether or not it can be a valid actor, object or activity ID.
func ValidateID(id string) (ID, error) {
	parsed, err := ParseID(id)
	if err != nil {
		return ID{}, err
	}

	if parsed.Kind != PortableID && parsed.Scheme != "https" {
		return ID{}, fmt.Errorf("%w: invalid scheme in %s", ErrInvalidID, id)
	}

	if parsed.hasUser {
		return ID{}, fmt.Errorf("%w: user in %s", ErrInvalidID, id)
	}

	if parsed.query != "" {
		return ID{}, fmt.Errorf("%w: query in %s", ErrInvalidID, id)
	}

	if strings.Contains(parsed.path, "/..") {
		return ID{}, fmt.Errorf("%w: invalid path in %s", ErrInvalidID, id)
	}

	return parsed, nil
}

// IsPortable determines whether or not an ActivityPub ID is portable.
func IsPortable(id string) bool {
	parsed, err := ParseID(id)
	return err == nil && parsed.Kind != URLID
}

// Canonical returns an ID in canonical form: if portable, it's converted to an ap:// URI without location hints.
func Canonical(id string) string {
	if parsed, err := ParseID(id); err == nil {
		return parsed.Canonical
	}

	return id
}

// SameID determines whether or not two ActivityPub IDs are equivalent.
func SameID(a, b string) bool {
	return Canonical(a) == Canonical(b)
}

// Gateway returns a https:// gateway URL for a portable ActivityPub ID.
func Gateway(gw, id string) string {
	if parsed, err := ParseID(id); err == nil && parsed.Kind != URLID {
		return gw + "/.well-known/apgateway/" + parsed.Canonical[len("ap://"):]
	}

	return id
}

// WithGateways returns an ap:// URI with @gateway location hints.
func WithGateways(id string, gateways []string) string {
	parsed, err := ParseID(id)
	if err != nil || parsed.Kind != PortableID || len(gateways) == 0 {
		return id
	}

	head, fragment := splitFragment(parsed.Canonical)

	var b strings.Builder
	b.WriteString(head)
	for i, gw := range gateways {
		if i == 0 {
			b.WriteByte('?')
		} else {
			b.WriteByte('&')
		}
		b.WriteString(gatewayParam)
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(gw))
	}
	b.WriteString(fragment)

	return b.String()
}

// Origins returns the origin and the host of an ActivityPub ID.
func Origins(id string) (string, string, error) {
	parsed, err := ParseID(id)
	if err != nil {
		return "", "", err
	}

	return parsed.Origin, parsed.Host, nil
}

// Origin returns the origin of an ActivityPub ID.
func Origin(id string) (string, error) {
	parsed, err := ParseID(id)
	return parsed.Origin, err
}
