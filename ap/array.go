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

package ap

import "encoding/json"

// Array is an array or a single item.
type Array[T any] []T

func (a *Array[T]) UnmarshalJSON(b []byte) error {
	var tmp []T
	if err := json.Unmarshal(b, &tmp); err != nil {
		tmp = make([]T, 1)
		if err := json.Unmarshal(b, &tmp[0]); err != nil {
			return err
		}
	}

	*a = tmp
	return nil
}

func (a Array[T]) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte("[]"), nil
	}

	return json.Marshal(([]T)(a))
}
