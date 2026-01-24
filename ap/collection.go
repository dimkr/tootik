/*
Copyright 2025, 2026 Dima Krasner

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

type (
	CollectionType     string
	CollectionPageType string
)

const (
	OrderedCollection     CollectionType     = "OrderedCollection"
	OrderedCollectionPage CollectionPageType = "OrderedCollectionPage"
)

// Collection represents an ActivityPub collection.
type Collection struct {
	Context      any            `json:"@context"`
	ID           string         `json:"id"`
	Type         CollectionType `json:"type"`
	First        string         `json:"first,omitempty"`
	Last         string         `json:"last,omitempty"`
	TotalItems   *int64         `json:"totalItems,omitempty"`
	OrderedItems any            `json:"orderedItems,omitzero"`
}

// CollectionPage represents a [Collection] page.
type CollectionPage struct {
	Context      any                `json:"@context"`
	ID           string             `json:"id"`
	Type         CollectionPageType `json:"type"`
	Next         string             `json:"next,omitempty"`
	Prev         string             `json:"prev,omitempty"`
	PartOf       string             `json:"partOf,omitempty"`
	OrderedItems any                `json:"orderedItems,omitzero"`
}
