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

package gemtext

import (
	"fmt"
	"github.com/dimkr/tootik/text"
)

type Writer struct {
	text.Base
}

func (w *Writer) Title(title string) {
	w.Write([]byte("# "))
	w.Write([]byte(title))
	w.Write([]byte("\n\n"))
}

func (w *Writer) Titlef(format string, a ...any) {
	w.Write([]byte("# "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\n\n"))
}

func (w *Writer) Subtitle(subtitle string) {
	w.Write([]byte("## "))
	w.Write([]byte(subtitle))
	w.Write([]byte("\n\n"))
}

func (w *Writer) Subtitlef(format string, a ...any) {
	w.Write([]byte("## "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte("\n\n"))
}

func (w *Writer) Text(line string) {
	w.Write([]byte(line))
	w.Write([]byte{'\n'})
}

func (w *Writer) Textf(format string, a ...any) {
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *Writer) Empty() {
	w.Write([]byte{'\n'})
}

func (w *Writer) Link(url, name string) {
	fmt.Fprintf(w, "=> %s ", url)
	w.Write([]byte(name))
	w.Write([]byte{'\n'})
}

func (w *Writer) Linkf(url, format string, a ...any) {
	fmt.Fprintf(w, "=> %s ", url)
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *Writer) Item(item string) {
	w.Write([]byte("* "))
	w.Write([]byte(item))
	w.Write([]byte{'\n'})
}

func (w *Writer) Itemf(format string, a ...any) {
	w.Write([]byte("* "))
	fmt.Fprintf(w, format, a...)
	w.Write([]byte{'\n'})
}

func (w *Writer) Quote(quote string) {
	w.Write([]byte("> "))
	w.Write([]byte(quote))
	w.Write([]byte{'\n'})
}

func (w *Writer) Raw(alt, raw string) {
	fmt.Fprintf(w, "```%s\n", alt)
	w.Write([]byte(raw))
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		w.Write([]byte{'\n'})
	}
	w.Write([]byte("```\n"))
}

func (w *Writer) Separator() {
	w.Write([]byte("\n────\n\n"))
}
