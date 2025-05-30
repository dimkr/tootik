/*
Copyright 2023 - 2025 Dima Krasner

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
	"fmt"
	"io"

	"github.com/dimkr/tootik/front/text"
)

type writer struct {
	*text.LineWriter
}

// Wrap wraps an [io.Writer] with a Gemini response writer.
func Wrap(inner io.Writer) text.Writer {
	return &writer{text.LineBuffered(inner)}
}

func (w *writer) Status(code int, meta string) {
	fmt.Fprintf(w, "%d %s\r\n", code, meta)
	if code != 20 {
		w.Flush()
	}
}

func (w *writer) Statusf(code int, format string, a ...any) {
	fmt.Fprintf(w, "%d ", code)
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\r\n"))
	if code != 20 {
		w.Flush()
	}
}

func (w *writer) OK() {
	w.Status(20, "text/gemini")
}

func (w *writer) Error() {
	w.Status(40, "Error")
}

func (w *writer) Redirect(link string) {
	w.Status(30, link)
}

func (w *writer) Redirectf(format string, a ...any) {
	w.Statusf(30, format, a...)
}

func (w *writer) Title(title string) {
	w.Write([]byte("# "))
	w.Write([]byte(title))
	w.Write([]byte("\n\n"))
}

func (w *writer) Titlef(format string, a ...any) {
	w.Write([]byte("# "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\n\n"))
}

func (w *writer) Subtitle(subtitle string) {
	w.Write([]byte("## "))
	w.Write([]byte(subtitle))
	w.Write([]byte("\n\n"))
}

func (w *writer) Subtitlef(format string, a ...any) {
	w.Write([]byte("## "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\n\n"))
}

func (w *writer) Text(line string) {
	w.Write([]byte(line))
	w.Write([]byte{'\n'})
}

func (w *writer) Textf(format string, a ...any) {
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *writer) Empty() {
	w.Write([]byte{'\n'})
}

func (w *writer) Link(url, name string) {
	fmt.Fprintf(w, "=> %s ", url)
	w.Write([]byte(name))
	w.Write([]byte{'\n'})
}

func (w *writer) Linkf(url, format string, a ...any) {
	fmt.Fprintf(w, "=> %s ", url)
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *writer) Item(item string) {
	w.Write([]byte("* "))
	w.Write([]byte(item))
	w.Write([]byte{'\n'})
}

func (w *writer) Itemf(format string, a ...any) {
	w.Write([]byte("* "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *writer) Quote(quote string) {
	w.Write([]byte("> "))
	w.Write([]byte(quote))
	w.Write([]byte{'\n'})
}

func (w *writer) Quotef(format string, a ...any) {
	w.Write([]byte("> "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *writer) Raw(alt, raw string) {
	fmt.Fprintf(w, "```%s\n", alt)
	w.Write([]byte(raw))
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		w.Write([]byte{'\n'})
	}
	w.Write([]byte("```\n"))
}

func (w *writer) Separator() {
	w.Write([]byte("\n────\n\n"))
}

func (*writer) Clone(w io.Writer) text.Writer {
	return Wrap(w)
}
