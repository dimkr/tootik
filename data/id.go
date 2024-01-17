/*
Copyright 2024 Dima Krasner

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

package data

import (
	"net/url"
	"strings"
)

// IsIDValid determines whether or not a string can be a valid actor, object or activity ID.
func IsIDValid(id string) bool {
	if id == "" {
		return false
	}

	u, err := url.Parse(id)
	if err != nil {
		return false
	}

	if u.Scheme != "https" {
		return false
	}

	if u.User != nil {
		return false
	}

	if u.RawQuery != "" {
		return false
	}

	if strings.Contains(u.Path, "/..") {
		return false
	}

	return true
}
