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
)

type Move struct {
	From, To, Captured Coord
}

var (
	ErrWait           = errors.New("not your turn")
	ErrImpossibleMove = errors.New("impossible move")
	ErrMustCapture    = errors.New("must capture")
)

func yieldKingMovesDir(kingPos Coord, dx, dy int, us, them map[Coord]Piece, yield func(Move) bool) bool {
	pos := kingPos
	captured := Coord{}

	for ((dx > 0 && pos.X < 7) || (dx < 0 && pos.X > 0)) && ((dy > 0 && pos.Y < 7) || (dy < 0 && pos.Y > 0)) {
		pos.X += dx
		pos.Y += dy

		if _, ok := us[pos]; ok {
			// movement is blocked by our piece
			break
		}

		if _, ok := them[pos]; ok {
			if captured != (Coord{}) {
				// movement is blocked by a captured piece
				break
			}

			captured = pos
			continue
		}

		if !yield(Move{From: kingPos, To: pos, Captured: captured}) {
			return false
		}
	}

	return true
}

func yieldKingMoves(kingPos Coord, us, them map[Coord]Piece, yield func(Move) bool) bool {
	if !yieldKingMovesDir(kingPos, 1, 1, us, them, yield) {
		return false
	}

	if !yieldKingMovesDir(kingPos, -1, 1, us, them, yield) {
		return false
	}

	if !yieldKingMovesDir(kingPos, 1, -1, us, them, yield) {
		return false
	}

	if !yieldKingMovesDir(kingPos, -1, -1, us, them, yield) {
		return false
	}

	return true
}

func doCaptureMove(
	pos, next, captured Coord,
	ours, theirs map[Coord]Piece,
	yield func(Move) bool,
) bool {
	if _, ok := theirs[captured]; ok {
		if _, ok := ours[next]; !ok {
			if _, ok := theirs[next]; !ok {
				return yield(Move{From: pos, To: next, Captured: captured})
			}
		}
	}

	return true
}

func yieldCaptureMoves(pos Coord, ours, theirs map[Coord]Piece, yield func(Move) bool) bool {
	if pos.X < 6 && pos.Y < 6 && !doCaptureMove(
		pos, Coord{pos.X + 2, pos.Y + 2}, Coord{pos.X + 1, pos.Y + 1},
		ours, theirs,
		yield,
	) {
		return false
	}

	if pos.X > 1 && pos.Y < 6 && !doCaptureMove(
		pos, Coord{pos.X - 2, pos.Y + 2}, Coord{pos.X - 1, pos.Y + 1},
		ours, theirs,
		yield,
	) {
		return false
	}

	if pos.X > 1 && pos.Y > 1 && !doCaptureMove(
		pos, Coord{pos.X - 2, pos.Y - 2}, Coord{pos.X - 1, pos.Y - 1},
		ours, theirs,
		yield,
	) {
		return false
	}

	if pos.X < 6 && pos.Y > 1 && !doCaptureMove(
		pos, Coord{pos.X + 2, pos.Y - 2}, Coord{pos.X + 1, pos.Y - 1},
		ours, theirs,
		yield,
	) {
		return false
	}

	return true
}

func yieldMove(
	pos, next Coord,
	ours, theirs map[Coord]Piece,
	yield func(Move) bool,
) bool {
	if _, ok := ours[next]; !ok {
		if _, ok := theirs[next]; !ok {
			return yield(Move{From: pos, To: next})
		}
	}

	return true
}

func (s *State) OrcMoves() iter.Seq[Move] {
	return func(yield func(Move) bool) {
		for pos, orc := range s.Orcs {
			if orc.King {
				if !yieldKingMoves(pos, s.Orcs, s.Humans, yield) {
					return
				}

				continue
			}

			if !yieldCaptureMoves(pos, s.Orcs, s.Humans, yield) {
				return
			}

			if pos.X < 7 && pos.Y < 7 && !yieldMove(pos, Coord{pos.X + 1, pos.Y + 1}, s.Orcs, s.Humans, yield) {
				return
			}

			if pos.X > 0 && pos.Y < 7 && !yieldMove(pos, Coord{pos.X - 1, pos.Y + 1}, s.Orcs, s.Humans, yield) {
				return
			}
		}
	}
}

func (s *State) HumanMoves() iter.Seq[Move] {
	return func(yield func(Move) bool) {
		for pos, human := range s.Humans {
			if human.King {
				if !yieldKingMoves(pos, s.Humans, s.Orcs, yield) {
					return
				}

				continue
			}

			if !yieldCaptureMoves(pos, s.Humans, s.Orcs, yield) {
				return
			}

			if pos.X > 0 && pos.Y > 0 && !yieldMove(pos, Coord{pos.X - 1, pos.Y - 1}, s.Humans, s.Orcs, yield) {
				return
			}

			if pos.X < 7 && pos.Y > 0 && !yieldMove(pos, Coord{pos.X + 1, pos.Y - 1}, s.Humans, s.Orcs, yield) {
				return
			}
		}
	}
}

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

	return ErrImpossibleMove

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
	})

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
