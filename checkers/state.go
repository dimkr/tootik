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

type State struct {
	Board
	Turns   []Board
	Current Player
}

func Start() *State {
	return &State{
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
		},
		Current: Human,
	}
}

func (s State) getCell(c Coord) Player {
	if _, ok := s.Humans[c]; ok {
		return Human
	}

	if _, ok := s.Orcs[c]; ok {
		return Orc
	}

	return None
}
