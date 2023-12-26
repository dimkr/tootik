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

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeUnmarshal_RFC3339(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"content":"a","time":"2023-12-19T16:05:27Z"}`), &s))
	assert.Equal(t, "a", s.Content)
	assert.Equal(t, Time{Time: time.Date(2023, time.December, 19, 16, 5, 27, 0, time.UTC)}, s.Time)
}

func TestTimeUnmarshal_RFC3339Nano(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"content":"a","time":"2023-12-19T16:05:13.330654Z"}`), &s))
	assert.Equal(t, "a", s.Content)
	assert.Equal(t, Time{Time: time.Date(2023, time.December, 19, 16, 5, 13, 330654000, time.UTC)}, s.Time)
}

func TestTimeUnmarshal_Threads(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"content":"a","time":"2023-12-23T22:25:02-0800"}`), &s))
	assert.Equal(t, "a", s.Content)
	assert.Equal(t, Time{Time: time.Date(2023, time.December, 23, 22, 25, 2, 0, time.FixedZone("", -8*60*60))}, s.Time)
}

func TestTimeUnmarshal_Null(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"content":"a","time":null}`), &s))
	assert.Equal(t, "a", s.Content)
	assert.Equal(t, Time{Time: time.Time{}}, s.Time)
}

func TestTimeUnmarshal_Missing(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.NoError(t, json.Unmarshal([]byte(`{"content":"a"}`), &s))
	assert.Equal(t, "a", s.Content)
	assert.Equal(t, Time{Time: time.Time{}}, s.Time)
	assert.Equal(t, Time{}, s.Time)
}

func TestTimeUnmarshal_Empty(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.Error(t, json.Unmarshal([]byte(`{"content":"a","time":""}`), &s))
}

func TestTimeUnmarshal_Object(t *testing.T) {
	var s struct {
		Content string `json:"content"`
		Time    Time   `json:"time"`
	}
	assert.Error(t, json.Unmarshal([]byte(`{"content":{"a"},"time":{}}`), &s))
}
