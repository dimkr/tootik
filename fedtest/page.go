/*
Copyright 2024 Dima Krasner

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

package fedtest

import (
	"crypto/tls"
	"slices"
	"strings"
)

// Respnonse represents a frontend page displayed to the user.
type Page struct {
	Request string
	Raw     string
	Status  string
	Lines   []Line
	Links   map[string]string

	cert   tls.Certificate
	server *Server
}

func parseResponse(s *Server, cert tls.Certificate, req, resp string) Page {
	end := strings.Index(resp, "\r\n")
	if end == -1 {
		return Page{}
	}

	lines := []Line{}
	links := map[string]string{}

	preformatted := false

	if len(resp) > end+2 {
		for _, line := range strings.Split(resp[end+2:], "\n") {
			if strings.HasPrefix(line, "```") {
				preformatted = !preformatted
			} else if preformatted {
				lines = append(lines, Line{Type: Preformatted, Text: line})
			} else if strings.HasPrefix(line, "=> ") {
				i := strings.IndexByte(line[3:], ' ')
				lines = append(lines, Line{Type: Link, Text: line[4+i:], URL: line[3 : 3+i]})
				links[line[4+i:]] = line[3 : 3+i]
			} else if strings.HasPrefix(line, "# ") {
				lines = append(lines, Line{Type: Heading, Text: line[2:]})
			} else if strings.HasPrefix(line, "## ") {
				lines = append(lines, Line{Type: SubHeading, Text: line[3:]})
			} else if strings.HasPrefix(line, "* ") {
				lines = append(lines, Line{Type: Item, Text: line[2:]})
			} else if strings.HasPrefix(line, "> ") {
				lines = append(lines, Line{Type: Quote, Text: line[2:]})
			} else {
				lines = append(lines, Line{Type: Text, Text: line})
			}
		}
	}

	return Page{
		Request: req,
		Raw:     resp,
		Status:  resp[:end],
		Lines:   lines,
		Links:   links,

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

func (p Page) Contains(line Line) Page {
	if !slices.Contains(p.Lines, line) {
		p.server.Test.Fatalf(`%s does not contain "%s" line`, p.Raw, line.Text)
	}

	return p
}

func (p Page) NotContains(line Line) Page {
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
