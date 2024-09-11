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

// Package guppy builds Guppy responses.
package guppy

import (
	"fmt"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/gmi"
	"io"
)

type Writer struct {
	text.Writer
	seq int
}

// Wrap wraps an [io.Writer] with a Guppy response writer.
func Wrap(w io.Writer, seq int) *Writer {
	return &Writer{Writer: gmi.Wrap(w), seq: seq}
}

func (w *Writer) Status(code int, meta string) {
	if code == w.seq {
		fmt.Fprintf(w, "%d %s\r\n", code, meta)
	} else if code == 3 || code == 4 {
		fmt.Fprintf(w, "%d %s\r\n", code, meta)
		w.Flush()
	} else if code == 10 {
		fmt.Fprintf(w, "1 Input required: %s\r\n", meta)
		w.Flush()
	} else {
		fmt.Fprintf(w, "4 %s\r\n", meta)
		w.Flush()
	}
}

func (w *Writer) Statusf(code int, format string, a ...any) {
	w.Status(code, fmt.Sprintf(format, a...))
}

func (w *Writer) OK() {
	w.Status(w.seq, "text/gemini")
}

func (w *Writer) Error() {
	w.Status(4, "Error")
}

func (w *Writer) Redirect(link string) {
	w.Status(3, link)
}

func (w *Writer) Redirectf(format string, a ...any) {
	w.Statusf(3, format, a...)
}

func (w *Writer) Clone(w2 io.Writer) text.Writer {
	return Wrap(w2, w.seq)
}
