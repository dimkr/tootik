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

import "time"

type ObjectType string

const (
	NoteObject    ObjectType = "Note"
	PageObject    ObjectType = "Page"
	ArticleObject ObjectType = "Article"
)

type Object struct {
	Context      any          `json:"@context,omitempty"`
	ID           string       `json:"id"`
	Type         ObjectType   `json:"type"`
	AttributedTo string       `json:"attributedTo,omitempty"`
	InReplyTo    string       `json:"inReplyTo,omitempty"`
	Content      string       `json:"content,omitempty"`
	Name         string       `json:"name,omitempty"`
	Published    time.Time    `json:"published,omitempty"`
	Updated      time.Time    `json:"updated,omitempty"`
	To           Audience     `json:"to,omitempty"`
	CC           Audience     `json:"cc,omitempty"`
	Tag          []Mention    `json:"tag,omitempty"`
	Attachment   []Attachment `json:"attachment,omitempty"`
	URL          string       `json:"url,omitempty"`
}

func (o *Object) IsPublic() bool {
	return o.To.Contains(Public) || o.CC.Contains(Public)
}
