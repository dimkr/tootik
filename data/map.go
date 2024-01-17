/*
Copyright 2023, 2024 Dima Krasner

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

// OrderedMap is a map that maintains insertion order. Listing of keys (using [OrderedMap.Keys]) iterates over keys and allocates memory.
type OrderedMap[TK comparable, TV any] map[TK]valueAndIndex[TV]

// Contains determines if the map contains a key.
func (m OrderedMap[TK, TV]) Contains(key TK) bool {
	_, contains := m[key]
	return contains
}

// Store adds a key/value pair to the map if the map doesn't contain it already.
func (m OrderedMap[TK, TV]) Store(key TK, value TV) {
	if _, dup := m[key]; !dup {
		m[key] = valueAndIndex[TV]{value, len(m)}
	}
}

// Keys returns a list of keys in the map.
// To do so, it iterates over keys and allocates memory.
func (m OrderedMap[TK, TV]) Keys() []TK {
	l := make([]TK, len(m))

	for k, v := range m {
		l[v.index] = k
	}

	return l
}

// Range iterates over the map and calls a callback for each key/value pair.
// Iteration stops if the callback returns false.
// Range calls [OrderedMap.Keys], therefore it allocates memory.
func (m OrderedMap[TK, TV]) Range(f func(key TK, value TV) bool) {
	for _, k := range m.Keys() {
		if !f(k, m[k].value) {
			break
		}
	}
}
