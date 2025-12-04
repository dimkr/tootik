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

package ap

import "testing"

// https://codeberg.org/fediverse/fep/src/commit/480415584237eb19cb7373b6a25faa6fa6e3a200/fep/521b/fep-521b.md
func Test_FEP521b(t *testing.T) {
	if !KeyRegex.MatchString("u7QGwDY2Tjn93PVFWWq02piP1NE9_XRlg-c8-jhJiDqKBDw") {
		t.Fatalf("Failed to detect base64-encoded key")
	}

	if !KeyRegex.MatchString("z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2") {
		t.Fatalf("Failed to detect base58-encoded key")
	}
}
