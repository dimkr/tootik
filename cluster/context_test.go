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

package cluster

import (
	"testing"

	"github.com/dimkr/tootik/gemtext"
)

func TestCluster_Context(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()

	a := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "a").
		OK()
	cluster.Settle(t)

	b := a.
		FollowInput("💬 Reply", "b").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "b"})

	c := b.
		FollowInput("💬 Reply", "c").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c"})

	d := c.
		FollowInput("💬 Reply", "d").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"})

	e := d.
		FollowInput("💬 Reply", "e").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	f := e.
		FollowInput("💬 Reply", "f").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "f"})

	i := f.
		FollowInput("💬 Reply", "g").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g"}).
		FollowInput("💬 Reply", "h").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "h"}).
		FollowInput("💬 Reply", "i").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "i"})
	cluster.Settle(t)

	cluster["b.localdomain"].Config.PostContextDepth = 5

	bob.
		Goto(i.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[3 replies]", URL: d.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "f"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "h"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "i"})

	cluster["b.localdomain"].Config.PostContextDepth = 4

	bob.
		Goto(i.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[4 replies]", URL: e.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "e"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "f"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "h"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "i"})

	bob.
		Goto(f.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[1 reply]", URL: b.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	bob.
		Goto(e.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	cluster["b.localdomain"].Config.PostContextDepth--

	bob.
		Goto(e.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[1 reply]", URL: b.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	cluster["b.localdomain"].Config.PostContextDepth--

	bob.
		Goto(e.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[2 replies]", URL: c.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	cluster["b.localdomain"].Config.PostContextDepth--

	bob.
		Goto(e.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[3 replies]", URL: d.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	bob.
		Goto(d.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[2 replies]", URL: c.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e"})

	bob.
		Goto(c.Path).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a"}).
		Contains(gemtext.Line{Type: gemtext.Link, Text: "[1 reply]", URL: b.Path}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "d"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "e"})
}
