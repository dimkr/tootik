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
	"slices"
	"strings"
	"testing"
)

type Response struct {
	URL    string
	Raw    string
	Status string
	Lines  []Line

	test *testing.T
}

func ParseResponse(t *testing.T, url, resp string) Response {
	end := strings.Index(resp, "\r\n")
	if end == -1 {
		return Response{}
	}

	lines := []Line{}

	preformetted := false

	if len(resp) > end+2 {
		for _, line := range strings.Split(resp[end+2:], "\n") {
			if line == "```" {
				preformetted = !preformetted
			} else if preformetted {
				lines = append(lines, Line{Type: Preformatted, Text: line})
			} else if strings.HasPrefix(line, "=> ") {
				i := strings.IndexByte(line[3:], ' ')
				lines = append(lines, Line{Type: Link, Text: line[4+i:], URL: line[3 : 3+i]})
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

	return Response{
		URL:    url,
		Raw:    resp,
		Status: resp[:end],
		Lines:  lines,

		test: t,
	}
}

func (r Response) GetURL(text string) string {
	var url string
	for _, line := range r.Lines {
		if line.Type == Link && line.Text == text {
			url = line.URL
		}
	}

	if url == "" {
		r.test.Fatalf(`%s does not contain "%s" link`, r.Raw, text)
	}

	return url
}

func (r Response) AssertOK() Response {
	if r.Status != "20 text/gemini" {
		r.test.Fatalf(`status is not OK: %s`, r.Status)
	}

	return r
}

func (r Response) AssertContains(line Line) Response {
	if !slices.Contains(r.Lines, line) {
		r.test.Fatalf(`%s does not contain "%s" line`, r.Raw, line.Text)
	}

	return r
}

func (r Response) AssertNotContains(line Line) Response {
	if slices.Contains(r.Lines, line) {
		r.test.Fatalf(`%s does contains "%s" line`, r.Raw, line.Text)
	}

	return r
}
