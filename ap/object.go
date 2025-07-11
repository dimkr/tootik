/*
Copyright 2023 - 2025 Dima Krasner

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
)

type ObjectType string

const (
	Note      ObjectType = "Note"
	Page      ObjectType = "Page"
	Article   ObjectType = "Article"
	Question  ObjectType = "Question"
	Tombstone ObjectType = "Tombstone"
)

// Object represents most ActivityPub objects.
// Actors are represented by [Actor].
type Object struct {
	Context      any          `json:"@context,omitempty"`
	ID           string       `json:"id"`
	Type         ObjectType   `json:"type"`
	AttributedTo string       `json:"attributedTo,omitempty"`
	InReplyTo    string       `json:"inReplyTo,omitempty"`
	Content      string       `json:"content,omitempty"`
	Summary      string       `json:"summary,omitempty"`
	Sensitive    bool         `json:"sensitive,omitempty"`
	Name         string       `json:"name,omitempty"`
	Published    Time         `json:"published,omitzero"`
	Updated      Time         `json:"updated,omitzero"`
	To           Audience     `json:"to,omitzero"`
	CC           Audience     `json:"cc,omitzero"`
	Audience     string       `json:"audience,omitempty"`
	Tag          Array[Tag]   `json:"tag,omitzero"`
	Attachment   []Attachment `json:"attachment,omitempty"`
	URL          string       `json:"url,omitempty"`

	// polls
	VotersCount int64        `json:"votersCount,omitempty"`
	OneOf       []PollOption `json:"oneOf,omitempty"`
	AnyOf       []PollOption `json:"anyOf,omitempty"`
	EndTime     Time         `json:"endTime,omitzero"`
	Closed      Time         `json:"closed,omitzero"`
}

func (o *Object) IsPublic() bool {
	return o.To.Contains(Public) || o.CC.Contains(Public)
}

func (o *Object) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, o)
	case string:
		return json.Unmarshal([]byte(v), o)
	default:
		return fmt.Errorf("unsupported conversion from %T to %T", src, o)
	}
}

func (o *Object) Value() (driver.Value, error) {
	buf, err := json.Marshal(o)
	return string(buf), err
}
