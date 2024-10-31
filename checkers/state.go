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
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type State struct {
	Board
	Turns   []Board `json:"turns,omitempty"`
	Current Player  `json:"current"`
}

func Start(first Player) *State {
	state := &State{
		Board: Board{Humans: map[Coord]Piece{
			{1, 7}: {ID: 8},
			{3, 7}: {ID: 9},
			{5, 7}: {ID: 10},
			{7, 7}: {ID: 11},
			{0, 6}: {ID: 4},
			{2, 6}: {ID: 5},
			{4, 6}: {ID: 6},
			{6, 6}: {ID: 7},
			{1, 5}: {ID: 0},
			{3, 5}: {ID: 1},
			{5, 5}: {ID: 2},
			{7, 5}: {ID: 3},
		},
			Orcs: map[Coord]Piece{
				{0, 0}: {ID: 8},
				{2, 0}: {ID: 9},
				{4, 0}: {ID: 10},
				{6, 0}: {ID: 11},
				{1, 1}: {ID: 4},
				{3, 1}: {ID: 5},
				{5, 1}: {ID: 6},
				{7, 1}: {ID: 7},
				{0, 2}: {ID: 0},
				{2, 2}: {ID: 1},
				{4, 2}: {ID: 2},
				{6, 2}: {ID: 3},
			},
			Moved: time.Now(),
		},
		Current: first,
	}

	return state
}

func (s *State) Scan(src any) error {
	str, ok := src.(string)
	if !ok {
		return fmt.Errorf("unsupported conversion from %T to %T", src, s)
	}
	return json.Unmarshal([]byte(str), s)
}

func (s *State) Value() (driver.Value, error) {
	buf, err := json.Marshal(s)
	return string(buf), err
}
