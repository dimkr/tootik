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

package cluster

import (
	"testing"

	"github.com/dimkr/tootik/gemtext"
)

func TestMetadata_Whitespace(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "my website=it's http://localhost.localdomain").
		Contains(gemtext.Line{Type: gemtext.Link, URL: "http://localhost.localdomain", Text: "my website: it's http://localhost.localdomain"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Link, URL: "http://localhost.localdomain", Text: "my website: it's http://localhost.localdomain"})

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		Follow("➖ Remove").
		NotContains(gemtext.Line{Type: gemtext.Link, URL: "http://localhost.localdomain", Text: "my website: it's http://localhost.localdomain"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(gemtext.Line{Type: gemtext.Link, URL: "http://localhost.localdomain", Text: "my website: it's http://localhost.localdomain"})
}

func TestMetadata_LineBreak(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "a=b").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		FollowInput("➕ Add", "c=d\ne").
		Error("40 Bad input")

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"})
}

func TestMetadata_Link(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "my website=http://localhost.localdomain/index.html").
		Contains(gemtext.Line{Type: gemtext.Link, Text: "my website", URL: "http://localhost.localdomain/index.html"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Link, Text: "my website", URL: "http://localhost.localdomain/index.html"})
}

func TestMetadata_HTML(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", `HTML tags like <p>=<a>link</a><br/>`).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "HTML tags like &lt;p&gt;: <a>link</a><br/>"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "HTML tags like &lt;p&gt;: <a>link</a><br/>"})
}

func TestMetadata_Equals(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "a=b=c").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b=c"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b=c"})
}

func TestMetadata_Add(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "a=b").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		FollowInput("➕ Add", "c=d").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		FollowInput("➕ Add", "c=d").
		Error("40 Cannot add metadata field")

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"})
}

func TestMetadata_Maximum(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "a=b").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		FollowInput("➕ Add", "c=d").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		FollowInput("➕ Add", "e=f").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		FollowInput("➕ Add", "g=h").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"}).
		OK()

	bob.
		GotoInput("/users/metadata/add", "i=j").
		Error("40 Reached the maximum number of metadata fields")

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "i: j"})
}

func TestMetadata_Remove(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		FollowInput("➕ Add", "a=b").
		FollowInput("➕ Add", "c=d").
		FollowInput("➕ Add", "e=f").
		FollowInput("➕ Add", "g=h").
		OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"})

	list := bob.
		Follow("⚙️ Settings").
		Follow("💳 Metadata").
		OK()

	list = list.
		Goto("/users/metadata/remove?g")

	list.
		OK()

	list.
		Goto("/users/metadata/remove?g").
		Error("40 Field does not exist")

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"})

	list = list.
		Follow("➖ Remove").
		OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"})

	list.
		Follow("➖ Remove").
		Follow("➖ Remove").
		OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "a: b"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "c: d"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "e: f"}).
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "g: h"})
}
