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
)

func Bars(keys []string, values []int64) string {
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}

	unit := float64(8)
	if max >= 16 {
		unit = float64(max) / 8
	}

	var w bytes.Buffer

	for i := 0; i < len(keys); i++ {
		var bar [8]rune
		for j, v := 0, float64(values[i]); j < 8; j, v = j+1, v-unit {
			if v >= unit {
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
			} else if v >= unit*3/8 {
				bar[j] = '▎'
			} else if v >= unit*2/8 {
				bar[j] = '▎'
			} else if v >= unit*1/8 {
				bar[j] = '▏'
			} else {
				bar[j] = ' '
			}
		}
		fmt.Fprintf(&w, "%s %s %10d\n", keys[i], string(bar[:]), values[i])
	}

	return w.String()
}
