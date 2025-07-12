/*
Copyright 2024, 2025 Dima Krasner

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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrayUnmarshal_Empty(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"a","tag":[]}`), &o))
	assert.Equal(t, "a", o.ID)
	assert.Equal(t, 0, len(o.Tag))
}

func TestArrayUnmarshal_OneTag(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"a","tag":{"type":"Hashtag","name":"b"}}`), &o))
	assert.Equal(t, "a", o.ID)
	assert.Equal(t, 1, len(o.Tag))
	assert.Equal(t, Hashtag, o.Tag[0].Type)
	assert.Equal(t, "b", o.Tag[0].Name)
}

func TestArrayUnmarshal_OneTagInArray(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"a","tag":[{"type":"Hashtag","name":"b"}]}`), &o))
	assert.Equal(t, "a", o.ID)
	assert.Equal(t, 1, len(o.Tag))
	assert.Equal(t, Hashtag, o.Tag[0].Type)
	assert.Equal(t, "b", o.Tag[0].Name)
}

func TestArrayUnmarshal_TwoTagsInArray(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"a","tag":[{"type":"Hashtag","name":"b"},{"type":"Emoji","name":"c"}]}`), &o))
	assert.Equal(t, "a", o.ID)
	assert.Equal(t, 2, len(o.Tag))
	assert.Equal(t, Hashtag, o.Tag[0].Type)
	assert.Equal(t, "b", o.Tag[0].Name)
	assert.Equal(t, Emoji, o.Tag[1].Type)
	assert.Equal(t, "c", o.Tag[1].Name)
}

func TestArrayUnmarshal_String(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.Error(t, json.Unmarshal([]byte(`{"id":"a","tag":"b"}`), &o))
}

func TestArrayUnmarshal_Null(t *testing.T) {
	var o struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"a","tag":null}`), &o))
	assert.Equal(t, "a", o.ID)
	assert.Equal(t, 0, len(o.Tag))
}

func TestArrayMarshal_Null(t *testing.T) {
	o := struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}{
		ID:  "a",
		Tag: nil,
	}
	j, err := json.Marshal(o)
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a","tag":[]}`, string(j))
}

func TestArrayMarshal_Empty(t *testing.T) {
	o := struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}{
		ID:  "a",
		Tag: []Tag{},
	}
	j, err := json.Marshal(o)
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a","tag":[]}`, string(j))
}

func TestArrayMarshal_OneTag(t *testing.T) {
	o := struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}{
		ID: "a",
		Tag: []Tag{
			{
				Type: Hashtag,
				Name: "b",
			},
		},
	}
	j, err := json.Marshal(o)
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a","tag":[{"type":"Hashtag","name":"b"}]}`, string(j))
}

func TestArrayMarshal_TwoTags(t *testing.T) {
	o := struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag"`
	}{
		ID: "a",
		Tag: []Tag{
			{
				Type: Hashtag,
				Name: "b",
			},
			{
				Type: Emoji,
				Name: "c",
			},
		},
	}
	j, err := json.Marshal(o)
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a","tag":[{"type":"Hashtag","name":"b"},{"type":"Emoji","name":"c"}]}`, string(j))
}

func TestArrayMarshal_NilOmitZero(t *testing.T) {
	j, err := json.Marshal(struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag,omitzero"`
	}{
		ID: "a",
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a"}`, string(j))
}

func TestArrayMarshal_EmptyOmitZero(t *testing.T) {
	j, err := json.Marshal(struct {
		ID  string     `json:"id"`
		Tag Array[Tag] `json:"tag,omitzero"`
	}{
		ID:  "a",
		Tag: Array[Tag]{},
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"id":"a"}`, string(j))
}
