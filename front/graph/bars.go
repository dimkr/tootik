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

package graph

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

func Bars(keys []string, values []int64) string {
	flip := false
	var keyWidth int
	for i, key := range keys {
		w := utf8.RuneCountInString(key)
		/*
			if keys have different lengths, put them on the right: the graph is
			misaligned if labels contain emojis but viewed through a problematic
			terminal emulator or if emoji fonts are old or missing; we must put
			labels on the right to ensure that everything else is aligned
		*/
		if i > 0 && w > 0 && w != keyWidth {
			flip = true
			break
		}
		if w > keyWidth {
			keyWidth = w
		}
	}

	valueStrings := make([]string, len(values))

	var valueWidth int
	var max int64
	for i, v := range values {
		if v > max {
			max = v
		}
		s := fmt.Sprintf("%d", v)
		valueStrings[i] = s
		l := len(s)
		if l > valueWidth {
			valueWidth = l
		}
	}

	unit := float64(max) / 8

	var w bytes.Buffer

	for i := 0; i < len(keys); i++ {
		if keys[i] == "" {
			continue
		}

		var bar [8]rune
		for j, v := 0, float64(values[i]); j < 8; j, v = j+1, v-unit {
			if unit == 0 {
				bar[j] = ' '
			} else if v >= unit {
				bar[j] = '█'
			} else if v >= unit*7/8 {
				bar[j] = '▉'
			} else if v >= unit*6/8 {
				bar[j] = '▊'
			} else if v >= unit*5/8 {
				bar[j] = '▋'
			} else if v >= unit*4/8 {
				bar[j] = '▌'
			} else if v >= unit*3/8 {
				bar[j] = '▍'
			} else if v >= unit*2/8 {
				bar[j] = '▎'
			} else if v >= unit*1/8 {
				bar[j] = '▏'
			} else {
				bar[j] = ' '
			}
		}

		if flip {
			fmt.Fprintf(&w, "%-*s %8s %s\n", valueWidth, valueStrings[i], string(bar[:]), keys[i])
		} else {
			fmt.Fprintf(&w, "%-*s %8s %d\n", keyWidth, keys[i], string(bar[:]), values[i])
		}
	}

	return w.String()
}
