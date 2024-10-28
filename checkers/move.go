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
	"slices"
)

type Move struct {
	From, To, Captured Coord
}

var (
	ErrWait           = errors.New("not your turn")
	ErrImpossibleMove = errors.New("impossible move")
	ErrMustCapture    = errors.New("must capture")
)

func (s *State) doKingMoves(kingPos Coord, dx, dy int, us, them map[Coord]Piece, yield func(Move) bool) bool {
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

func (s *State) kingMoves(kingPos Coord, us, them map[Coord]Piece, yield func(Move) bool) bool {
	if !s.doKingMoves(kingPos, 1, 1, us, them, yield) {
		return false
	}

	if !s.doKingMoves(kingPos, -1, 1, us, them, yield) {
		return false
	}

	if !s.doKingMoves(kingPos, 1, -1, us, them, yield) {
		return false
	}

	if !s.doKingMoves(kingPos, -1, -1, us, them, yield) {
		return false
	}

	return true
}

func (s *State) doCaptureMoves(pos Coord, enemy Player, yield func(Move) bool) bool {
	if pos.X < 6 && pos.Y < 6 {
		captured := Coord{pos.X + 1, pos.Y + 1}
		next := Coord{pos.X + 2, pos.Y + 2}
		if s.getCell(captured) == enemy && s.getCell(next) == None {
			if !yield(Move{From: pos, To: next, Captured: captured}) {
				return false
			}
		}
	}

	if pos.X > 1 && pos.Y < 6 {
		captured := Coord{pos.X - 1, pos.Y + 1}
		next := Coord{pos.X - 2, pos.Y + 2}
		if s.getCell(captured) == enemy && s.getCell(next) == None {
			if !yield(Move{From: pos, To: next, Captured: captured}) {
				return false
			}
		}
	}

	if pos.X > 1 && pos.Y > 1 {
		captured := Coord{pos.X - 1, pos.Y - 1}
		next := Coord{pos.X - 2, pos.Y - 2}
		if s.getCell(captured) == enemy && s.getCell(next) == None {
			if !yield(Move{From: pos, To: next, Captured: captured}) {
				return false
			}
		}
	}

	if pos.X < 6 && pos.Y > 1 {
		captured := Coord{pos.X + 1, pos.Y - 1}
		next := Coord{pos.X + 2, pos.Y - 2}
		if s.getCell(captured) == enemy && s.getCell(next) == None {
			if !yield(Move{From: pos, To: next, Captured: captured}) {
				return false
			}
		}
	}

	return true
}

func (s *State) OrcMoves() iter.Seq[Move] {
	return func(yield func(Move) bool) {
		for pos, orc := range s.Orcs {
			if orc.King {
				if !s.kingMoves(pos, s.Orcs, s.Humans, yield) {
					return
				}

				continue
			}

			if !s.doCaptureMoves(pos, Human, yield) {
				return
			}

			if pos.X < 7 && pos.Y < 7 {
				next := Coord{pos.X + 1, pos.Y + 1}
				if s.getCell(next) == None {
					if !yield(Move{From: pos, To: next}) {
						return
					}
				}
			}

			if pos.X > 0 && pos.Y < 7 {
				next := Coord{pos.X - 1, pos.Y + 1}
				if s.getCell(next) == None {
					if !yield(Move{From: pos, To: next}) {
						return
					}
				}
			}
		}
	}
}

func (s *State) HumanMoves() iter.Seq[Move] {
	return func(yield func(Move) bool) {
		for pos, human := range s.Humans {
			if human.King {
				if !s.kingMoves(pos, s.Humans, s.Orcs, yield) {
					return
				}

				continue
			}

			if !s.doCaptureMoves(pos, Orc, yield) {
				return
			}

			if pos.X > 0 && pos.Y > 0 {
				next := Coord{pos.X - 1, pos.Y - 1}
				if s.getCell(next) == None {
					if !yield(Move{From: pos, To: next}) {
						return
					}
				}
			}

			if pos.X < 7 && pos.Y > 0 {
				next := Coord{pos.X + 1, pos.Y - 1}
				if s.getCell(next) == None {
					if !yield(Move{From: pos, To: next}) {
						return
					}
				}
			}
		}
	}
}

func (s *State) ActHuman(move Move) error {
	if s.Current != Human {
		return ErrWait
	}

	moves := s.HumanMoves()

	for m := range moves {
		if m == move {
			goto legal
		}
	}

	return ErrImpossibleMove

legal:
	if move.Captured == (Coord{}) {
		for m := range moves {
			if m.Captured != (Coord{}) {
				return ErrMustCapture
			}
		}
	}

	s.Turns = append(s.Turns, Board{
		Humans: maps.Clone(s.Humans),
		Orcs:   maps.Clone(s.Orcs),
	})

	if move.Captured != (Coord{}) {
		delete(s.Orcs, move.Captured)
	}

	piece := s.Humans[move.From]
	if move.To.Y == 0 {
		piece.King = true
	}
	s.Humans[move.To] = piece
	delete(s.Humans, move.From)

	s.Current = Orc
	if move.Captured != (Coord{}) {
		for m := range s.HumanMoves() {
			if m.From == move.To && m.Captured != (Coord{}) {
				s.Current = Human
				break
			}
		}
	}

	return nil
}

func (s *State) ActOrc(move Move) error {
	if s.Current != Orc {
		return ErrWait
	}

	moves := slices.Collect(s.OrcMoves())

	for _, m := range moves {
		if m == move {
			goto legal
		}
	}

	return ErrImpossibleMove

legal:
	if move.Captured == (Coord{}) {
		for _, m := range moves {
			if m.Captured != (Coord{}) {
				return ErrMustCapture
			}
		}
	}

	s.Turns = append(s.Turns, Board{
		Humans: maps.Clone(s.Humans),
		Orcs:   maps.Clone(s.Orcs),
	})

	if move.Captured != (Coord{}) {
		delete(s.Humans, move.Captured)
	}

	piece := s.Orcs[move.From]
	if move.To.Y == 7 {
		piece.King = true
	}
	s.Orcs[move.To] = piece
	delete(s.Orcs, move.From)

	s.Current = Human
	if move.Captured != (Coord{}) {
		for m := range s.OrcMoves() {
			if m.From == move.To && m.Captured != (Coord{}) {
				s.Current = Orc
				break
			}
		}
	}

	return nil
}
