/*
Copyright 2026 Dima Krasner

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
	"crypto/sha256"
	"encoding/base64"
)

func Slug(id string) string {
	sum := sha256.Sum256([]byte(id))
	return base64.RawURLEncoding.EncodeToString(sum[:9])
}
