/*
Copyright 2023 - 2026 Dima Krasner

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

package text

import "github.com/mattn/go-runewidth"

// WordWrap wraps long lines.
func WordWrap(text string, width, maxLines int) []string {
	if text == "" {
		return []string{""}
	}

	var lines []string
	runes := []rune(text)
	start := 0

	for start < len(runes) && (maxLines == -1 || len(lines) <= maxLines) {
		lineWidth := 0
		lastSpace := -1
		lineRunes := 0

		for i := start; i < len(runes); i++ {
			if rw := runewidth.RuneWidth(runes[i]); lineWidth+rw > width {
				break
			} else {
				lineWidth += rw
			}

			lineRunes++
			if runes[i] == ' ' || runes[i] == '\t' {
				lastSpace = i
			}
		}

		if lineRunes == 0 {
			lines = append(lines, string(runes[start:start+1]))
			start++
			continue
		}

		if start+lineRunes < len(runes) && lastSpace > start {
			lines = append(lines, string(runes[start:lastSpace]))
			start = lastSpace + 1
		} else {
			lines = append(lines, string(runes[start:start+lineRunes]))
			start += lineRunes
		}
	}

	return lines
}
