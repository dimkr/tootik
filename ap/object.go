/*
Copyright 2023, 2024 Dima Krasner

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
	NoteObject     ObjectType = "Note"
	PageObject     ObjectType = "Page"
	ArticleObject  ObjectType = "Article"
	QuestionObject ObjectType = "Question"
)

// Object represents most ActivityPub objects.
// Actors are represented by Actor.
type Object struct {
	Context      any          `json:"@context,omitempty"`
	ID           string       `json:"id"`
	Type         ObjectType   `json:"type"`
	AttributedTo string       `json:"attributedTo,omitempty"`
	InReplyTo    string       `json:"inReplyTo,omitempty"`
	Content      string       `json:"content,omitempty"`
	Name         string       `json:"name,omitempty"`
	Published    Time         `json:"published"`
	Updated      *Time        `json:"updated,omitempty"`
	To           Audience     `json:"to,omitempty"`
	CC           Audience     `json:"cc,omitempty"`
	Audience     string       `json:"audience,omitempty"`
	Tag          []Mention    `json:"tag,omitempty"`
	Attachment   []Attachment `json:"attachment,omitempty"`
	URL          string       `json:"url,omitempty"`

	// polls
	VotersCount int64        `json:"votersCount,omitempty"`
	OneOf       []PollOption `json:"oneOf,omitempty"`
	AnyOf       []PollOption `json:"anyOf,omitempty"`
	EndTime     *Time        `json:"endTime,omitempty"`
	Closed      *Time        `json:"closed,omitempty"`
}

func (o *Object) IsPublic() bool {
	return o.To.Contains(Public) || o.CC.Contains(Public)
}

func (o *Object) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("unsupported conversion from %T to %T", src, o)
	}
	return json.Unmarshal([]byte(s), o)
}

func (o *Object) Value() (driver.Value, error) {
	buf, err := json.Marshal(o)
	return string(buf), err
}
