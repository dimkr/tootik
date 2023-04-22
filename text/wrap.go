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

package text

func WordWrap(text string, width, maxLines int) []string {
	if text == "" {
		return []string{""}
	}

	runes := []rune(text)
	var lines []string

	for i := 0; i < len(runes) && (maxLines == -1 || len(lines) <= maxLines); {
		if i >= len(runes)-width {
			lines = append(lines, string(runes[i:]))
			break
		}

		lastSpace := -1

		for j := i + width - 1; j > i; j-- {
			if runes[j] == ' ' || runes[j] == '\t' {
				lastSpace = j
				break
			}
		}

		if lastSpace >= 0 {
			lines = append(lines, string(runes[i:lastSpace]))
			i = lastSpace + 1
		} else {
			lines = append(lines, string(runes[i:i+width]))
			i += width
		}
	}

	return lines
}
