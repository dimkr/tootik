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

package gmap

import (
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/text"
	"io"
	"net/url"
	"strings"
)

const lineWidth = 70

type writer struct {
	text.Base
}

func Wrap(w io.Writer) text.Writer {
	return &writer{Base: text.Base{Writer: w}}
}

func (w *writer) Status(code int, meta string) {
	w.Textf("%d: %s", code, meta)
}

func (w *writer) Statusf(code int, format string, a ...any) {
	w.Status(code, fmt.Sprintf(format, a...))
}

func (w *writer) OK() {}

func (w *writer) Error() {
	w.Text("40: Error")
}

func (w *writer) wrap(t byte, prefix, name, selector, host, port string) {
	for _, line := range text.WordWrap(name, lineWidth-len(prefix), -1) {
		fmt.Fprintf(w, "%c%s%s\t%s\t%s\t%s\r\n", t, prefix, line, selector, host, port)
	}
}

func (w *writer) Redirect(link string) {
	w.Link(link, "Redirected to "+link)
}

func (w *writer) Redirectf(format string, a ...any) {
	w.Redirect(fmt.Sprintf(format, a...))
}

func (w *writer) Title(title string) {
	w.wrap('i', "# ", title, "/", "0", "0")
	w.Empty()
}

func (w *writer) Titlef(format string, a ...any) {
	w.Title(fmt.Sprintf(format, a...))
}

func (w *writer) Subtitle(subtitle string) {
	w.wrap('i', "## ", subtitle, "/", "0", "0")
	w.Empty()
}

func (w *writer) Subtitlef(format string, a ...any) {
	w.Subtitle(fmt.Sprintf(format, a...))
}

func (w *writer) Text(line string) {
	w.wrap('i', "", line, "/", "0", "0")
}

func (w *writer) Textf(format string, a ...any) {
	w.Text(fmt.Sprintf(format, a...))
}

func (w *writer) Empty() {
	w.wrap('i', "", "", "/", "0", "0")
}

func (w *writer) Link(link, name string) {
	if link[0] == '/' {
		w.wrap('1', "", name, link, cfg.Domain, "70")
	} else if u, err := url.Parse(link); err == nil {
		if u.Scheme == "gopher" {
			port := u.Port()
			if port == "" {
				w.wrap('1', "", name, u.Path, u.Host, "70")
			} else {
				w.wrap('1', "", name, u.Path, u.Host, port)
			}
		} else {
			w.wrap('h', "", name, "URL:"+link, "0", "0")
		}
	} else {
		w.wrap('h', "", name, "URL:"+link, "0", "0")
	}
}

func (w *writer) Linkf(link, format string, a ...any) {
	w.Link(link, fmt.Sprintf(format, a...))
}

func (w *writer) Itemf(format string, a ...any) {
	w.wrap('i', "* ", fmt.Sprintf(format, a...), "/", "0", "0")
}

func (w *writer) Quote(quote string) {
	w.wrap('i', "> ", quote, "/", "0", "0")
}

func (w *writer) Raw(alt, raw string) {
	for _, line := range strings.Split(raw, "\n") {
		w.Write([]byte{'i'})
		w.Write([]byte(line))
		w.Write([]byte("\t/\t0\t0\r\n"))
	}
}

func (w *writer) Separator() {
	w.Empty()
	w.Text("────")
	w.Empty()
}

func (_ *writer) Clone(w io.Writer) text.Writer {
	return &writer{Base: text.Base{Writer: w}}
}
