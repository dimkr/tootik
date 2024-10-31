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
	"errors"
	"iter"
	"maps"
	"time"
)

var (
	ErrWait        = errors.New("not your turn")
	ErrInvalidMove = errors.New("invalid move")
	ErrMustCapture = errors.New("must capture")
)

func (s *State) act(
	from, to Coord,
	us, them Player,
	ours, theirs map[Coord]Piece,
	moves func() iter.Seq[Move],
	kingY int,
) error {
	if s.Current != us {
		return ErrWait
	}

	var captured Coord

	for move := range moves() {
		if move.From == from && move.To == to {
			captured = move.Captured
			goto legal
		}
	}

	return ErrInvalidMove

legal:
	if captured == (Coord{}) {
		for m := range moves() {
			if m.Captured != (Coord{}) {
				return ErrMustCapture
			}
		}
	}

	s.Turns = append(s.Turns, Board{
		Humans: maps.Clone(s.Humans),
		Orcs:   maps.Clone(s.Orcs),
		Moved:  s.Moved,
	})
	s.Moved = time.Now()

	if captured != (Coord{}) {
		delete(theirs, captured)
	}

	piece := ours[from]
	if to.Y == kingY {
		piece.King = true
	}
	ours[to] = piece
	delete(ours, from)

	s.Current = them
	if captured != (Coord{}) {
		for m := range moves() {
			if m.From == to && m.Captured != (Coord{}) {
				s.Current = us
				break
			}
		}
	}

	return nil
}

func (s *State) ActHuman(from, to Coord) error {
	return s.act(
		from,
		to,
		Human,
		Orc,
		s.Humans,
		s.Orcs,
		s.HumanMoves,
		0,
	)
}

func (s *State) ActOrc(from, to Coord) error {
	return s.act(
		from,
		to,
		Orc,
		Human,
		s.Orcs,
		s.Humans,
		s.OrcMoves,
		7,
	)
}
