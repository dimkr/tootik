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

package checkers

import (
	"fmt"
	"strings"
)

type Board struct {
	Humans pieces `json:"humans,omitempty"`
	Orcs   pieces `json:"orcs,omitempty"`
}

func (s Board) String() string {
	var b strings.Builder

	b.Grow(421)
	b.WriteString(" ╔═╤═╤═╤═╤═╤═╤═╤═╗\n")

	for row := 7; row >= 0; row-- {
		fmt.Fprintf(&b, `%d║`, row)
		for col := 0; col < 8; col++ {
			if p, ok := s.Humans[Coord{col, row}]; ok {
				if p.King {
					b.WriteByte('H')
				} else {
					b.WriteByte('h')
				}
			} else if p, ok := s.Orcs[Coord{col, row}]; ok {
				if p.King {
					b.WriteByte('O')
				} else {
					b.WriteByte('o')
				}
			} else {
				b.WriteByte(' ')
			}
			if col < 7 {
				b.WriteRune('│')
			} else {
				b.WriteRune('║')
			}
		}
		if row > 0 {
			b.WriteString("\n ╟─┼─┼─┼─┼─┼─┼─┼─╢\n")
		} else {
			b.WriteByte('\n')
		}
	}

	b.WriteString(" ╚═╧═╧═╧═╧═╧═╧═╧═╝\n  0 1 2 3 4 5 6 7\n")

	return b.String()
}
