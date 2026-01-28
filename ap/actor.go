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
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/dimkr/tootik/danger"
)

type ActorType string

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

// Capabilities returns capabilities the actor advertises support for.
func (a *Actor) Capabilities() Capability {
	cap := Capability(0)

	for _, imp := range a.Generator.Implements {
		switch imp.Href {
		case "https://datatracker.ietf.org/doc/html/rfc9421":
			cap |= RFC9421RSASignatures
		case "https://datatracker.ietf.org/doc/html/rfc9421#name-eddsa-using-curve-edwards25":
			cap |= RFC9421Ed25519Signatures
		}
	}

	return cap
}
