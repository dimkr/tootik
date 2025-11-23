/*
Copyright 2025 Dima Krasner

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

package danger

import "unsafe"

// String casts a byte slice to a string without copying the underlying array.
//
// The caller must not modify b afterwards because this will change the returned string.
func String(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
