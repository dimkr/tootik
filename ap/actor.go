/*
Copyright 2023 Dima Krasner

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

type ActorType string

const (
	Person ActorType = "Person"
	Group  ActorType = "Group"
)

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
	Icon                      Attachment        `json:"icon,omitempty"`
	ManuallyApprovesFollowers bool              `json:"manuallyApprovesFollowers"`
	AlsoKnownAs               Audience          `json:"alsoKnownAs,omitempty"`
}
