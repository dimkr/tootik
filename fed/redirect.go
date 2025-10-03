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

package fed

import (
	"net/http"
	"strings"
)

func shouldRedirect(r *http.Request) bool {
	accept := strings.ReplaceAll(r.Header.Get("Accept"), " ", "")
	return (accept == "text/html" || strings.HasPrefix(accept, "text/html,") || strings.HasSuffix(accept, ",text/html") || strings.Contains(accept, ",text/html,")) && !strings.Contains(accept, "application/activity+json") && !strings.Contains(accept, `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
}
