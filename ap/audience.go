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

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/data"
)

// Audience is an ordered, unique list of actor IDs.
type Audience struct {
	data.OrderedMap[string, struct{}]
}

func (a *Audience) Add(s string) {
	a.OrderedMap.Store(s, struct{}{})
}

func (a *Audience) UnmarshalJSON(b []byte) error {
	var l []string
	if err := json.Unmarshal(b, &l); err != nil {
		// Mastodon represents poll votes as a Create with a string in "to"
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}

		a.OrderedMap = data.OrderedMap[string, struct{}]{}
		a.Add(s)

		return nil
	}

	a.OrderedMap = data.OrderedMap[string, struct{}]{}

	for _, s := range l {
		a.Add(s)
	}

	return nil
}

func (a Audience) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Keys())
}

func (a *Audience) Scan(src any) error {
	if src == nil {
		a.OrderedMap = data.OrderedMap[string, struct{}]{}
		return nil
	}

	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("unsupported conversion from %T to %T", src, a)
	}
	return json.Unmarshal([]byte(s), a)
}

func (a *Audience) Value() (driver.Value, error) {
	buf, err := json.Marshal(a)
	return string(buf), err
}
