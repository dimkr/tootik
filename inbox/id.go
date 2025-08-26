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
package inbox

import (
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/google/uuid"
)

// NewID generates a pseudo-random ID.
func (q *Queue) NewID(actorID, prefix string) (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate %s ID: %w", prefix, err)
	}

	if m := ap.CompatibleURLRegex.FindStringSubmatch(actorID); m != nil {
		return fmt.Sprintf("https://%s/.well-known/apgateway/did:key:%s/actor/%s/%s", q.Domain, m[1], prefix, u.String()), nil
	}

	return fmt.Sprintf("https://%s/%s/%s", q.Domain, prefix, u.String()), nil
}
