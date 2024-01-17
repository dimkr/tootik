/*
Copyright 2023, 2024 Dima Krasner

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

package text

import "io"

// Writer builds a textual response.
type Writer interface {
	io.Writer

	Clone(io.Writer) Writer
	Unwrap() io.Writer

	OK()
	Error()
	Redirect(string)
	Redirectf(string, ...any)
	Status(int, string)
	Statusf(int, string, ...any)
	Title(string)
	Titlef(string, ...any)
	Subtitle(string)
	Subtitlef(string, ...any)
	Text(string)
	Textf(string, ...any)
	Empty()
	Link(string, string)
	Linkf(string, string, ...any)
	Item(string)
	Itemf(string, ...any)
	Quote(string)
	Raw(string, string)
	Separator()
}
