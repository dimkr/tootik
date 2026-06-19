/*
Copyright 2024 - 2026 Dima Krasner

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

package cluster

import (
	"crypto/tls"
	"slices"

	"github.com/dimkr/tootik/front/text/gmi"
)

// Respnonse represents a frontend page displayed to the user.
type Page struct {
	Path   string
	Raw    string
	Status string
	Lines  []gmi.Line
	Links  map[string]string

	cert   tls.Certificate
	server *Server
}

func parseResponse(s *Server, cert tls.Certificate, req, resp string) Page {
	status, lines, links := gmi.Parse(resp)

	return Page{
		Path:   req,
		Raw:    resp,
		Status: status,
		Lines:  lines,
		Links:  links,

		cert:   cert,
		server: s,
	}
}

func (p Page) OK() Page {
	if p.Status != "20 text/gemini" {
		p.server.Test.Fatalf(`status is not OK: %s`, p.Status)
	}

	return p
}

func (p Page) Error(err string) {
	if p.Status != err {
		p.server.Test.Fatalf(`unexpected status: %s`, p.Status)
	}
}

func (p Page) Contains(line gmi.Line) Page {
	if !slices.Contains(p.Lines, line) {
		p.server.Test.Fatalf(`%s does not contain "%s" line`, p.Raw, line.Text)
	}

	return p
}

func (p Page) NotContains(line gmi.Line) Page {
	if slices.Contains(p.Lines, line) {
		p.server.Test.Fatalf(`%s does contains "%s" line`, p.Raw, line.Text)
	}

	return p
}

// FollowInput is like [Page.Follow] but also accepts user-provided input.
func (p Page) FollowInput(text, input string) Page {
	path, ok := p.Links[text]
	if !ok {
		p.server.Test.Fatalf(`%s does not contain "%s" link`, p.Raw, text)
	}

	return p.server.HandleInput(p.cert, path, input)
}

// Follow follows a link, follows redirects and returns a [Page].
func (p Page) Follow(text string) Page {
	return p.FollowInput(text, "")
}

// Refresh fetches the same [Page] again.
func (p Page) Refresh() Page {
	return p.server.Handle(p.cert, p.Path)
}

// Goto navigates to a different page by its path.
func (p Page) Goto(path string) Page {
	return p.server.Handle(p.cert, path)
}

// GotoInput is like [Page.Goto] but also accepts user-provided input.
func (p Page) GotoInput(path, input string) Page {
	return p.server.HandleInput(p.cert, path, input)
}
