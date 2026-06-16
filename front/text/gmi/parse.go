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

package gmi

import "strings"

// Parse parses a Gemtext document.
func Parse(resp string) (string, []Line, map[string]string) {
	end := strings.Index(resp, "\r\n")
	if end == -1 {
		return "", nil, nil
	}

	lines := []Line{}
	links := map[string]string{}

	preformatted := false

	if len(resp) > end+2 {
		for line := range strings.SplitSeq(resp[end+2:], "\n") {
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

	return resp[:end], lines, links
}
