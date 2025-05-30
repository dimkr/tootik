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

// Package cluster contains complex tests that involve multiple servers.
package cluster

import (
	"testing"

	"github.com/dimkr/tootik/inbox"
)

// Cluster represents a collection of servers that talk to each other.
type Cluster Client

// NewCluster creates a collection of servers that talk to each other.
func NewCluster(t *testing.T, domain ...string) Cluster {
	t.Parallel()

	c := Client{}

	for _, d := range domain {
		c[d] = NewServer(t.Context(), t, d, c)
	}

	return Cluster(c)
}

// Settle waits until all servers are done processing queued activities, both incoming and outgoing.
func (c Cluster) Settle(t *testing.T) {
	for {
		again := false

		for d, server := range c {
			if n, err := server.Incoming.ProcessBatch(t.Context()); err != nil {
				server.Test.Fatalf("Failed to process incoming queue on %s: %v", d, err)
			} else if n > 0 {
				again = true
			}

			if n, err := server.Outgoing.ProcessBatch(t.Context()); err != nil {
				server.Test.Fatalf("Failed to process outgoing queue on %s: %v", d, err)
			} else if n > 0 {
				again = true
			}
		}

		if !again {
			break
		}
	}

	for d, server := range c {
		if err := (inbox.FeedUpdater{Domain: d, Config: server.Config, DB: server.DB}).Run(t.Context()); err != nil {
			server.Test.Fatalf("Failed to update feeds on %s: %v", d, err)
		}
	}
}

// Stop stops all servers in the cluster.
func (c Cluster) Stop() {
	for _, s := range c {
		s.Stop()
	}
}
