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

// Package gmi build Gemini responses.
package gmi

import (
	"bufio"
	"fmt"
	"github.com/dimkr/tootik/front/text"
	"io"
)

type writer struct {
	*bufio.Writer
	discard bool
}

const bufferSize = 256

// Wrap wraps an [io.Writer] with a Gemini response writer.
func Wrap(w io.Writer) text.Writer {
	return &writer{Writer: bufio.NewWriterSize(w, bufferSize)}
}

func (w *writer) Unwrap() io.Writer {
	return w.Writer
}

func (w *writer) Status(code int, meta string) {
	fmt.Fprintf(w, "%d %s\r\n", code, meta)
	if code != 20 {
		w.Flush()
		w.discard = true
	}
}

func (w *writer) Statusf(code int, format string, a ...any) {
	fmt.Fprintf(w, "%d ", code)
	fmt.Fprintf(w, format, a...)
	w.WriteString("\r\n")
	if code != 20 {
		w.Flush()
		w.discard = true
	}
}

func (w *writer) OK() {
	w.Status(20, "text/gemini")
}

func (w *writer) Error() {
	w.Status(40, "Error")
}

func (w *writer) Redirect(link string) {
	w.Statusf(30, link)
}

func (w *writer) Redirectf(format string, a ...any) {
	w.Statusf(30, format, a...)
}

func (w *writer) Title(title string) {
	if w.discard {
		return
	}

	w.WriteString("# ")
	w.WriteString(title)
	w.WriteString("\n\n")
}

func (w *writer) Titlef(format string, a ...any) {
	if w.discard {
		return
	}

	w.WriteString("# ")
	fmt.Fprintf(w, format, a...)
	w.WriteString("\n\n")
}

func (w *writer) Subtitle(subtitle string) {
	if w.discard {
		return
	}

	w.WriteString("## ")
	w.WriteString(subtitle)
	w.WriteString("\n\n")
}

func (w *writer) Subtitlef(format string, a ...any) {
	if w.discard {
		return
	}

	w.WriteString("## ")
	fmt.Fprintf(w, format, a...)
	w.WriteString("\n\n")
}

func (w *writer) Text(line string) {
	if w.discard {
		return
	}

	w.WriteString(line)
	w.WriteRune('\n')
}

func (w *writer) Textf(format string, a ...any) {
	if w.discard {
		return
	}

	fmt.Fprintf(w, format, a...)
	w.WriteRune('\n')
}

func (w *writer) Empty() {
	if w.discard {
		return
	}

	w.WriteRune('\n')
}

func (w *writer) Link(url, name string) {
	if w.discard {
		return
	}

	fmt.Fprintf(w, "=> %s ", url)
	w.WriteString(name)
	w.WriteRune('\n')
}

func (w *writer) Linkf(url, format string, a ...any) {
	if w.discard {
		return
	}

	fmt.Fprintf(w, "=> %s ", url)
	fmt.Fprintf(w, format, a...)
	w.WriteRune('\n')
}

func (w *writer) Item(item string) {
	if w.discard {
		return
	}

	w.WriteString("* ")
	w.WriteString(item)
	w.WriteRune('\n')
}

func (w *writer) Itemf(format string, a ...any) {
	if w.discard {
		return
	}

	w.WriteString("* ")
	fmt.Fprintf(w, format, a...)
	w.WriteRune('\n')
}

func (w *writer) Quote(quote string) {
	if w.discard {
		return
	}

	w.WriteString("> ")
	w.WriteString(quote)
	w.WriteRune('\n')
}

func (w *writer) Raw(alt, raw string) {
	if w.discard {
		return
	}

	fmt.Fprintf(w, "```%s\n", alt)
	w.WriteString(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		w.WriteRune('\n')
	}
	w.WriteString("```\n")
}

func (w *writer) Separator() {
	if w.discard {
		return
	}

	w.WriteString("\n────\n\n")
}

func (*writer) Clone(w io.Writer) text.Writer {
	return &writer{Writer: bufio.NewWriterSize(w, bufferSize)}
}
