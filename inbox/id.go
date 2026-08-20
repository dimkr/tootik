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

package inbox

import (
	"fmt"

	"uuid"

	"github.com/dimkr/tootik/ap"
)

// NewID generates a pseudo-random ID.
func (inbox *Inbox) NewID(actorID, prefix string) string {
	u := uuid.NewV7()

	if m := ap.GatewayURLRegex.FindStringSubmatch(actorID); m != nil {
		return fmt.Sprintf("https://%s/.well-known/apgateway/did:key:%s/actor/%s/%s", inbox.Domain, m[1], prefix, u.String())
	}

	return fmt.Sprintf("https://%s/%s/%s", inbox.Domain, prefix, u.String())
}
