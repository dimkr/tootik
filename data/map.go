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

import "iter"

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

// Keys iterates over keys in the map.
// It allocates memory.
func (m OrderedMap[TK, TV]) Keys() iter.Seq[TK] {
	l := make([]*TK, len(m))

	return func(yield func(TK) bool) {
		next := 0

		for k, v := range m {
			if l[next] != nil {
				if !yield(*l[next]) {
					break
				}
				next++
			} else if v.index == next {
				if !yield(k) {
					break
				}
				next++
			} else {
				l[v.index] = &k
			}
		}
	}
}

// Values iterates over values in the map.
// Values calls [OrderedMap.Keys], therefore it allocates memory.
func (m OrderedMap[TK, TV]) Values() iter.Seq[TV] {
	return func(yield func(TV) bool) {
		for k := range m.Keys() {
			if !yield(m[k].value) {
				break
			}
		}
	}
}

// All iterates over the map and calls a callback for each key/value pair.
// All calls [OrderedMap.Keys], therefore it allocates memory.
func (m OrderedMap[TK, TV]) All() iter.Seq2[TK, TV] {
	return func(yield func(TK, TV) bool) {
		for k := range m.Keys() {
			if !yield(k, m[k].value) {
				break
			}
		}
	}
}
