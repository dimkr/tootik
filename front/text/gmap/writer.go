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

// Package gmap builds gophermaps.
package gmap

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
)

type writer struct {
	*text.LineWriter
	Domain string
	Config *cfg.Config
}

// Wrap wraps an [io.Writer] with a gophermap writer.
func Wrap(w io.Writer, domain string, cfg *cfg.Config) text.Writer {
	return &writer{LineWriter: text.LineBuffered(w), Domain: domain, Config: cfg}
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

func (w *writer) wrap(t byte, prefix, cont, name, selector, host, port string) {
	lines := text.WordWrap(name, w.Config.LineWidth-len(prefix), -1)

	if len(lines) > 0 {
		fmt.Fprintf(w, "%c%s%s\t%s\t%s\t%s\r\n", t, prefix, lines[0], selector, host, port)
	}

	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%c%s%s\t%s\t%s\t%s\r\n", t, cont, line, selector, host, port)
	}
}

func (w *writer) Redirect(link string) {
	w.Link(link, "Redirected to "+link)
}

func (w *writer) Redirectf(format string, a ...any) {
	w.Redirect(fmt.Sprintf(format, a...))
}

func (w *writer) Title(title string) {
	w.wrap('i', "# ", "  ", title, "/", "0", "0")
	w.Empty()
}

func (w *writer) Titlef(format string, a ...any) {
	w.Title(fmt.Sprintf(format, a...))
}

func (w *writer) Subtitle(subtitle string) {
	w.wrap('i', "## ", "   ", subtitle, "/", "0", "0")
	w.Empty()
}

func (w *writer) Subtitlef(format string, a ...any) {
	w.Subtitle(fmt.Sprintf(format, a...))
}

func (w *writer) Text(line string) {
	w.wrap('i', "", "", line, "/", "0", "0")
}

func (w *writer) Textf(format string, a ...any) {
	w.Text(fmt.Sprintf(format, a...))
}

func (w *writer) Empty() {
	w.wrap('i', "", "", "", "/", "0", "0")
}

func (w *writer) Link(link, name string) {
	if link[0] == '/' {
		w.wrap('1', "", "", name, link, w.Domain, "70")
	} else if u, err := url.Parse(link); err == nil {
		if u.Scheme == "gopher" {
			port := u.Port()
			if port == "" {
				w.wrap('1', "", "", name, u.Path, u.Host, "70")
			} else {
				w.wrap('1', "", "", name, u.Path, u.Host, port)
			}
		} else {
			w.wrap('h', "", "", name, "URL:"+link, "0", "0")
		}
	} else {
		w.wrap('h', "", "", name, "URL:"+link, "0", "0")
	}
}

func (w *writer) Linkf(link, format string, a ...any) {
	w.Link(link, fmt.Sprintf(format, a...))
}

func (w *writer) Item(item string) {
	w.wrap('i', "* ", "  ", item, "/", "0", "0")
}

func (w *writer) Itemf(format string, a ...any) {
	w.wrap('i', "* ", "  ", fmt.Sprintf(format, a...), "/", "0", "0")
}

func (w *writer) Quote(quote string) {
	w.wrap('i', "> ", "> ", quote, "/", "0", "0")
}

func (w *writer) Quotef(format string, a ...any) {
	w.wrap('i', "> ", "  ", fmt.Sprintf(format, a...), "/", "0", "0")
}

func (w *writer) Raw(alt, raw string) {
	end := len(raw)
	if raw[end-1] == '\n' {
		end -= 1
	}
	for line := range strings.SplitSeq(raw[:end], "\n") {
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

func (gw *writer) Clone(w io.Writer) text.Writer {
	return Wrap(w, gw.Domain, gw.Config)
}
