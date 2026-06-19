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

	"github.com/dimkr/tootik/front/text/gmi"
)

func TestBio_Set(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("📜 Bio").
		Contains(gmi.Line{Type: gmi.Text, Text: "Bio is empty."}).
		FollowInput("Set", "hello world\nthis is my bio").
		NotContains(gmi.Line{Type: gmi.Text, Text: "Bio is empty."}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello world"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "this is my bio"})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello world"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "this is my bio"})
}
