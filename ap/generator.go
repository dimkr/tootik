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

// Implement is a [Generator] capability.
type Implement struct {
	Href string `json:"href,omitempty"`
	Name string `json:"name,omitempty"`
}

// Generator generates [Object] objects.
type Generator struct {
	Type       ActorType        `json:"type"`
	Implements Array[Implement] `json:"implements,omitzero"`
}
