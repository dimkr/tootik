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
)

type jsonPiece struct {
	Piece
	Coord
}

type jsonBoard struct {
	Humans []jsonPiece
	Orcs   []jsonPiece
}

type stateJson struct {
	jsonBoard
	Turns   []jsonBoard
	Current Player
}

func (s *State) MarshalJSON() ([]byte, error) {
	tmp := stateJson{
		jsonBoard: jsonBoard{
			Humans: make([]jsonPiece, 0, len(s.Humans)),
			Orcs:   make([]jsonPiece, 0, len(s.Orcs)),
		},
		Current: s.Current,
	}

	for pos, human := range s.Humans {
		tmp.Humans = append(tmp.Humans, jsonPiece{Piece: human, Coord: pos})
	}

	for pos, orc := range s.Orcs {
		tmp.Orcs = append(tmp.Orcs, jsonPiece{Piece: orc, Coord: pos})
	}

	for pos, human := range s.Humans {
		tmp.Humans = append(tmp.Humans, jsonPiece{Piece: human, Coord: pos})
	}

	for _, turn := range s.Turns {
		clone := jsonBoard{
			Humans: make([]jsonPiece, 0, len(turn.Humans)),
			Orcs:   make([]jsonPiece, 0, len(turn.Orcs)),
		}

		for pos, human := range turn.Humans {
			clone.Humans = append(clone.Humans, jsonPiece{Piece: human, Coord: pos})
		}

		for pos, orc := range turn.Orcs {
			clone.Orcs = append(clone.Orcs, jsonPiece{Piece: orc, Coord: pos})
		}

		tmp.Turns = append(tmp.Turns, clone)
	}

	return json.Marshal(tmp)
}

func (s *State) UnmarshalJSON(b []byte) error {
	var tmp stateJson
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	s.Humans = make(map[Coord]Piece, 12)
	for _, human := range tmp.Humans {
		s.Humans[human.Coord] = human.Piece
	}

	s.Orcs = make(map[Coord]Piece, 12)
	for _, orc := range tmp.Orcs {
		s.Orcs[orc.Coord] = orc.Piece
	}

	s.Turns = make([]Board, 0, len(tmp.Turns))
	for _, clone := range tmp.Turns {
		board := Board{
			Humans: make(map[Coord]Piece, 12),
			Orcs:   make(map[Coord]Piece, 12),
		}

		for _, human := range clone.Humans {
			board.Humans[human.Coord] = human.Piece
		}

		for _, orc := range clone.Orcs {
			board.Orcs[orc.Coord] = orc.Piece
		}

		s.Turns = append(s.Turns, board)
	}

	s.Current = tmp.Current

	return nil
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
