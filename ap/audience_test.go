/*
Copyright 2025 Dima Krasner

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

	"github.com/dimkr/tootik/data"
)

func TestAudienceMarshal_Happyflow(t *testing.T) {
	to := Audience{}
	to.Add("x")
	to.Add("y")
	to.Add("y")

	if j, err := json.Marshal(struct {
		ID string   `json:"id"`
		To Audience `json:"to,omitzero"`
	}{
		ID: "a",
		To: to,
	}); err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	} else if string(j) != `{"id":"a","to":["x","y"]}` {
		t.Fatalf("Unexpected result: %s", string(j))
	}
}

func TestAudienceMarshal_NilOmitZero(t *testing.T) {
	if j, err := json.Marshal(struct {
		ID string   `json:"id"`
		To Audience `json:"to,omitzero"`
	}{
		ID: "a",
	}); err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	} else if string(j) != `{"id":"a"}` {
		t.Fatalf("Unexpected result: %s", string(j))
	}
}

func TestAudienceMarshal_NilMapOmitZero(t *testing.T) {
	if j, err := json.Marshal(struct {
		ID string   `json:"id"`
		To Audience `json:"to,omitzero"`
	}{
		ID: "a",
		To: Audience{},
	}); err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	} else if string(j) != `{"id":"a"}` {
		t.Fatalf("Unexpected result: %s", string(j))
	}
}

func TestAudienceMarshal_EmptyOmitZero(t *testing.T) {
	if j, err := json.Marshal(struct {
		ID string   `json:"id"`
		To Audience `json:"tag,omitzero"`
	}{
		ID: "a",
		To: Audience{
			OrderedMap: data.OrderedMap[string, struct{}]{},
		},
	}); err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	} else if string(j) != `{"id":"a"}` {
		t.Fatalf("Unexpected result: %s", string(j))
	}
}
