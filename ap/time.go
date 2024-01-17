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

package ap

import "time"

// Time is a wrapper around time.Time with fallback if parsing of RFC3339 fails
type Time struct {
	time.Time
}

func (t *Time) UnmarshalJSON(b []byte) error {
	err := t.Time.UnmarshalJSON(b)
	// ugly hack for Threads
	if err != nil && len(b) > 2 && b[0] == '"' && b[len(b)-1] == '"' {
		t.Time, err = time.Parse("2006-01-02T15:04:05-0700", string(b[1:len(b)-1]))
	}
	return err
}
