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

type MentionType string

const (
	MentionMention MentionType = "Mention"
	HashtagMention MentionType = "Hashtag"
	EmojiMention   MentionType = "Emoji"
)

type Mention struct {
	Type MentionType `json:"type,omitempty"`
	Name string      `json:"name,omitempty"`
	Href string      `json:"href,omitempty"`
	Icon *Attachment `json:"icon,omitempty"`
}

type Mentions []Mention

func (l Mentions) Contains(m Mention) bool {
	for _, m2 := range l {
		if m2.Name == m.Name && m2.Href == m.Href && m2.Type == m.Type {
			return true
		}
	}

	return false
}
