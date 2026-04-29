/*
Copyright 2023 - 2026 Dima Krasner

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
	"crypto/ed25519"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
)

type ActorType string

var didKeyVerificationMethodRegex = regexp.MustCompile(`^did:key:(z6Mk[a-km-zA-HJ-NP-Z1-9]+|u7Q[A-Za-z0-9_-]+)#(z6Mk[a-km-zA-HJ-NP-Z1-9]+|u7Q[A-Za-z0-9_-]+)$`)

const (
	Person      ActorType = "Person"
	Group       ActorType = "Group"
	Application ActorType = "Application"
	Service     ActorType = "Service"
)

// Actor represents an ActivityPub actor.
type Actor struct {
	Context                   any               `json:"@context"`
	ID                        string            `json:"id"`
	Type                      ActorType         `json:"type"`
	Inbox                     string            `json:"inbox"`
	Outbox                    string            `json:"outbox"`
	Endpoints                 map[string]string `json:"endpoints,omitempty"`
	PreferredUsername         string            `json:"preferredUsername"`
	Name                      string            `json:"name,omitempty"`
	Summary                   string            `json:"summary,omitempty"`
	Followers                 string            `json:"followers,omitempty"`
	PublicKey                 PublicKey         `json:"publicKey"`
	Icon                      Array[Attachment] `json:"icon,omitempty"`
	Image                     *Attachment       `json:"image,omitempty"`
	ManuallyApprovesFollowers bool              `json:"manuallyApprovesFollowers"`
	AlsoKnownAs               Audience          `json:"alsoKnownAs,omitzero"`
	Published                 Time              `json:"published,omitzero"`
	Updated                   Time              `json:"updated,omitzero"`
	MovedTo                   string            `json:"movedTo,omitempty"`
	Suspended                 bool              `json:"suspended,omitempty"`
	Attachment                []Attachment      `json:"attachment,omitempty"`
	AssertionMethod           []AssertionMethod `json:"assertionMethod,omitempty"`
	Implements                Array[Implement]  `json:"implements,omitzero"`
	Generator                 Generator         `json:"generator,omitzero"`
	Gateways                  []string          `json:"gateways,omitempty"`
	Proof                     Proof             `json:"proof,omitzero"`
}

func (a *Actor) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal(danger.Bytes(v), a)
	default:
		return fmt.Errorf("unsupported conversion from %T to %T", src, a)
	}
}

func (a *Actor) Value() (driver.Value, error) {
	return danger.MarshalJSON(a)
}

func (a *Actor) GetVerificationMethod(keyID string) (ed25519.PublicKey, error) {
	if m := didKeyVerificationMethodRegex.FindStringSubmatch(keyID); m != nil && m[1] == m[2] {
		raw, err := data.DecodeEd25519PublicKey(m[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", m[1], err)
		}

		return raw, nil
	}

	return a.GetKeyByID(keyID)
}

func (a *Actor) GetKeyByID(keyID string) (ed25519.PublicKey, error) {
	for _, key := range a.AssertionMethod {
		if key.ID != keyID {
			continue
		}

		if key.Type != "Multikey" {
			continue
		}

		if key.Controller != a.ID {
			continue
		}

		raw, err := data.DecodeEd25519PublicKey(key.PublicKeyMultibase)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", key.ID, err)
		}

		return raw, nil
	}

	return nil, fmt.Errorf("key %s does not exist", keyID)
}
