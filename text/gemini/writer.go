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

package gemini

import (
	"fmt"
	"github.com/dimkr/tootik/text"
	"github.com/dimkr/tootik/text/gemtext"
	"io"
)

type writer struct {
	gemtext.Writer
}

func Wrap(w io.Writer) text.Writer {
	return &writer{Writer: gemtext.Writer{Base: text.Base{Writer: w}}}
}

func (w *writer) Status(code int, meta string) {
	fmt.Fprintf(w, "%d %s\r\n", code, meta)
}

func (w *writer) Statusf(code int, format string, a ...any) {
	fmt.Fprintf(w, "%d ", code)
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\r\n"))
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

func (_ *writer) Clone(w io.Writer) text.Writer {
	return &writer{Writer: gemtext.Writer{Base: text.Base{Writer: w}}}
}
