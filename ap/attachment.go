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

type AttachmentType string

const (
	ImageAttachment AttachmentType = "Image"
)

type Attachment struct {
	Type      AttachmentType `json:"type,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
	URL       string         `json:"url,omitempty"`
	Href      string         `json:"href,omitempty"`
}
