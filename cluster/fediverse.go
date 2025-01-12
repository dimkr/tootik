/*
Copyright 2024, 2025 Dima Krasner

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

package cluster

import (
	"context"
	"testing"
)

// Fediverse represents a collection of servers that talk to each other.
type Fediverse Client

func NewFediverse(t *testing.T, domain ...string) Fediverse {
	t.Parallel()

	f := Client{}

	for _, d := range domain {
		f[d] = NewServer(context.Background(), t, d, f)
	}

	return Fediverse(f)
}

// Settle waits until all servers are done processing queued activities, both incoming and outgoing.
func (f Fediverse) Settle() {
	for {
		again := false

		for d, server := range f {
			if n, err := server.Incoming.ProcessBatch(context.Background()); err != nil {
				server.Test.Fatalf("Failed to process incoming queue on %s: %v", d, err)
			} else if n > 0 {
				again = true
			}

			if n, err := server.Outgoing.ProcessBatch(context.Background()); err != nil {
				server.Test.Fatalf("Failed to process outgoing queue on %s: %v", d, err)
			} else if n > 0 {
				again = true
			}
		}

		if !again {
			break
		}
	}
}

func (f Fediverse) Stop() {
	for _, s := range f {
		s.Stop()
	}
}
