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

	"github.com/dimkr/tootik/danger"
)

type AttachmentType string

const (
	Image         AttachmentType = "Image"
	PropertyValue AttachmentType = "PropertyValue"
)

type Attachment struct {
	Type      AttachmentType `json:"type,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
	URL       string         `json:"url,omitempty"`
	Href      string         `json:"href,omitempty"`
	Name      string         `json:"name,omitempty"`
	Val       string         `json:"value,omitempty"`
}

func (a *Attachment) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	default:
		return fmt.Errorf("unsupported conversion from %T to %T", src, a)
	}
}

func (a *Attachment) Value() (driver.Value, error) {
	return danger.MarshalJSON(a)
}
