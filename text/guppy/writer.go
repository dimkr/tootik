/*
Copyright 2023 Dima Krasner

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

package guppy

import (
	"fmt"
	"github.com/dimkr/tootik/text"
	"github.com/dimkr/tootik/text/gemtext"
	"io"
)

type Writer struct {
	gemtext.Writer
	seq int
}

func Wrap(w io.Writer, seq int) *Writer {
	return &Writer{Writer: gemtext.Writer{Base: text.Base{Writer: w}}, seq: seq}
}

func (w *Writer) Status(code int, meta string) {
	if code == 0 || code == 1 || code == w.seq {
		fmt.Fprintf(w, "%d %s\r\n", code, meta)
	} else {
		fmt.Fprintf(w, "1 %s\r\n", meta)
	}
}

func (w *Writer) Statusf(code int, format string, a ...any) {
	w.Status(code, fmt.Sprintf(format, a...))
}

func (w *Writer) OK() {
	w.Status(w.seq, "text/gemini")
}

func (w *Writer) Error() {
	w.Status(1, "Error")
}

func (w *Writer) Redirect(link string) {
	w.Status(0, link)
}

func (w *Writer) Redirectf(format string, a ...any) {
	w.Statusf(0, format, a...)
}

func (w *Writer) Clone(w2 io.Writer) text.Writer {
	return &Writer{Writer: gemtext.Writer{Base: text.Base{Writer: w2}}, seq: w.seq}
}
