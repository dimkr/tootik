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

package data

type valueAndIndex[TV any] struct {
	value TV
	index int
}

type OrderedMap[TK comparable, TV any] map[TK]valueAndIndex[TV]

func (m OrderedMap[TK, TV]) Contains(key TK) bool {
	_, contains := m[key]
	return contains
}

func (m OrderedMap[TK, TV]) Store(key TK, value TV) {
	if _, dup := m[key]; !dup {
		m[key] = valueAndIndex[TV]{value, len(m)}
	}
}

func (m OrderedMap[TK, TV]) Keys() []TK {
	l := make([]TK, len(m))

	for k, v := range m {
		l[v.index] = k
	}

	return l
}

func (m OrderedMap[TK, TV]) Range(f func(key TK, value TV) bool) {
	for _, k := range m.Keys() {
		if !f(k, m[k].value) {
			break
		}
	}
}
