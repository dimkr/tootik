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
	"slices"
	"testing"
)

func assertMoves(t *testing.T, moves, expected []Move) {
	if len(moves) != len(expected) {
		t.Fatalf("Wrong number of moves: %d instead of %d", len(moves), len(expected))
	}

	for i := range moves {
		if moves[i].From != expected[i].From {
			t.Fatalf("Wrong move %d: from %d,%d instead of %d,%d", i, moves[i].From.X, moves[i].From.Y, expected[i].From.X, expected[i].From.Y)
		}

		if moves[i].To != expected[i].To {
			t.Fatalf("Wrong move %d: to %d,%d instead of %d,%d", i, moves[i].To.X, moves[i].To.Y, expected[i].To.X, expected[i].To.Y)
		}

		if moves[i].Captured == (Coord{}) && expected[i].Captured == (Coord{}) {
			continue
		}

		if moves[i].Captured == (Coord{}) && expected[i].Captured != (Coord{}) {
			t.Fatalf("Wrong move %d: should capture %d,%d", i, expected[i].Captured.X, expected[i].Captured.Y)
		}

		if moves[i].Captured != (Coord{}) && expected[i].Captured == (Coord{}) {
			t.Fatalf("Wrong move %d: should not capture", i)
		}

		if moves[i].Captured != expected[i].Captured {
			t.Fatalf("Wrong move %d: should capture %d,%d instead of %d,%d", i, moves[i].Captured.X, moves[i].Captured.Y, expected[i].Captured.X, expected[i].Captured.Y)
		}
	}
}

func TestHuman_NoCapture(t *testing.T) {
	game := State{
		Board: Board{
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
			Orcs: map[Coord]Piece{
				Coord{0, 0}: Piece{},
			},
		},
		Current: Human,
	}

	moves := slices.Collect(game.HumanMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{3, 4},
				To:   Coord{2, 3},
			},
			{
				From: Coord{3, 4},
				To:   Coord{4, 3},
			},
		},
	)
}

func TestHuman_CaptureBottomLeft(t *testing.T) {
	game := State{
		Board: Board{
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
			Orcs: map[Coord]Piece{
				Coord{2, 3}: Piece{},
			},
		},
		Current: Human,
	}

	moves := slices.Collect(game.HumanMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{3, 4},
				To:       Coord{1, 2},
				Captured: Coord{2, 3},
			},
			{
				From: Coord{3, 4},
				To:   Coord{4, 3},
			},
		},
	)

	if err := game.ActHuman(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestHuman_CaptureBottomRight(t *testing.T) {
	game := State{
		Board: Board{
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
			Orcs: map[Coord]Piece{
				Coord{4, 3}: Piece{},
			},
		},
		Current: Human,
	}

	moves := slices.Collect(game.HumanMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{3, 4},
				To:       Coord{5, 2},
				Captured: Coord{4, 3},
			},
			{
				From: Coord{3, 4},
				To:   Coord{2, 3},
			},
		},
	)

	if err := game.ActHuman(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestHuman_CaptureTopLeft(t *testing.T) {
	game := State{
		Board: Board{
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{},
			},
		},
		Current: Human,
	}

	moves := slices.Collect(game.HumanMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{3, 4},
				To:       Coord{5, 6},
				Captured: Coord{4, 5},
			},
			{
				From: Coord{3, 4},
				To:   Coord{2, 3},
			},
			{
				From: Coord{3, 4},
				To:   Coord{4, 3},
			},
		},
	)

	if err := game.ActHuman(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[2].From, moves[2].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestHuman_CaptureTopRight(t *testing.T) {
	game := State{
		Board: Board{
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
			Orcs: map[Coord]Piece{
				Coord{2, 5}: Piece{},
			},
		},
		Current: Human,
	}

	moves := slices.Collect(game.HumanMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{3, 4},
				To:       Coord{1, 6},
				Captured: Coord{2, 5},
			},
			{
				From: Coord{3, 4},
				To:   Coord{2, 3},
			},
			{
				From: Coord{3, 4},
				To:   Coord{4, 3},
			},
		},
	)

	if err := game.ActHuman(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[2].From, moves[2].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActHuman(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_CaptureTopLeft(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{},
			},
			Humans: map[Coord]Piece{
				Coord{3, 6}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{4, 5},
				To:       Coord{2, 7},
				Captured: Coord{3, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{5, 6},
			},
		},
	)

	if err := game.ActOrc(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_NoCapture(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{6, 2}: Piece{},
			},
			Humans: map[Coord]Piece{
				Coord{1, 7}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{6, 2},
				To:   Coord{7, 3},
			},
			{
				From: Coord{6, 2},
				To:   Coord{5, 3},
			},
		},
	)

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_CaptureTopRight(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{},
			},
			Humans: map[Coord]Piece{
				Coord{5, 6}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{4, 5},
				To:       Coord{6, 7},
				Captured: Coord{5, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{3, 6},
			},
		},
	)

	if err := game.ActOrc(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_CaptureBottomLeft(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{},
			},
			Humans: map[Coord]Piece{
				Coord{3, 4}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{4, 5},
				To:       Coord{2, 3},
				Captured: Coord{3, 4},
			},
			{
				From: Coord{4, 5},
				To:   Coord{5, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{3, 6},
			},
		},
	)

	if err := game.ActOrc(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[2].From, moves[2].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_CaptureBottomRight(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{},
			},
			Humans: map[Coord]Piece{
				Coord{5, 4}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From:     Coord{4, 5},
				To:       Coord{6, 3},
				Captured: Coord{5, 4},
			},
			{
				From: Coord{4, 5},
				To:   Coord{5, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{3, 6},
			},
		},
	)

	if err := game.ActOrc(moves[1].From, moves[1].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[2].From, moves[2].To); err == nil {
		t.Fatal("Illegal move")
	} else if !errors.Is(err, ErrMustCapture) {
		t.Fatalf("Wrong reason: %v", err)
	}

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_KingCaptureBottomRight(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{3, 5}: Piece{King: true},
			},
			Humans: map[Coord]Piece{
				Coord{5, 3}: Piece{},
				Coord{2, 7}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{3, 5},
				To:   Coord{4, 6},
			},
			{
				From: Coord{3, 5},
				To:   Coord{5, 7},
			},
			{
				From: Coord{3, 5},
				To:   Coord{2, 6},
			},
			{
				From: Coord{3, 5},
				To:   Coord{1, 7},
			},
			{
				From: Coord{3, 5},
				To:   Coord{4, 4},
			},
			{
				From:     Coord{3, 5},
				To:       Coord{6, 2},
				Captured: Coord{5, 3},
			},
			{
				From:     Coord{3, 5},
				To:       Coord{7, 1},
				Captured: Coord{5, 3},
			},
			{
				From: Coord{3, 5},
				To:   Coord{2, 4},
			},
			{
				From: Coord{3, 5},
				To:   Coord{1, 3},
			},
			{
				From: Coord{3, 5},
				To:   Coord{0, 2},
			},
		},
	)

	for i := 0; i < len(moves); i++ {
		if moves[i].Captured == (Coord{}) {
			if err := game.ActOrc(moves[i].From, moves[i].To); err == nil {
				t.Fatalf("Illegal move %d", i)
			} else if !errors.Is(err, ErrMustCapture) {
				t.Fatalf("Wrong reason for move %d: %v", i, err)
			}
		}
	}

	if err := game.ActOrc(moves[5].From, moves[5].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_KingCaptureBottomLeft(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{4, 5}: Piece{King: true},
			},
			Humans: map[Coord]Piece{
				Coord{2, 3}: Piece{},
				Coord{6, 7}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{4, 5},
				To:   Coord{5, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{3, 6},
			},
			{
				From: Coord{4, 5},
				To:   Coord{2, 7},
			},
			{
				From: Coord{4, 5},
				To:   Coord{5, 4},
			},
			{
				From: Coord{4, 5},
				To:   Coord{6, 3},
			},
			{
				From: Coord{4, 5},
				To:   Coord{7, 2},
			},
			{
				From: Coord{4, 5},
				To:   Coord{3, 4},
			},
			{
				From:     Coord{4, 5},
				To:       Coord{1, 2},
				Captured: Coord{2, 3},
			},
			{
				From:     Coord{4, 5},
				To:       Coord{0, 1},
				Captured: Coord{2, 3},
			},
		},
	)

	for i := 0; i < len(moves); i++ {
		if moves[i].Captured == (Coord{}) {
			if err := game.ActOrc(moves[i].From, moves[i].To); err == nil {
				t.Fatalf("Illegal move %d", i)
			} else if !errors.Is(err, ErrMustCapture) {
				t.Fatalf("Wrong reason for move %d: %v", i, err)
			}
		}
	}

	if err := game.ActOrc(moves[7].From, moves[7].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_KingCaptureTopLeft(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{5, 4}: Piece{King: true},
			},
			Humans: map[Coord]Piece{
				Coord{3, 6}: Piece{},
				Coord{7, 2}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{5, 4},
				To:   Coord{6, 5},
			},
			{
				From: Coord{5, 4},
				To:   Coord{7, 6},
			},
			{
				From: Coord{5, 4},
				To:   Coord{4, 5},
			},
			{
				From:     Coord{5, 4},
				To:       Coord{2, 7},
				Captured: Coord{3, 6},
			},
			{
				From: Coord{5, 4},
				To:   Coord{6, 3},
			},
			{
				From: Coord{5, 4},
				To:   Coord{4, 3},
			},
			{
				From: Coord{5, 4},
				To:   Coord{3, 2},
			},
			{
				From: Coord{5, 4},
				To:   Coord{2, 1},
			},
			{
				From: Coord{5, 4},
				To:   Coord{1, 0},
			},
		},
	)

	for i := 0; i < len(moves); i++ {
		if moves[i].Captured == (Coord{}) {
			if err := game.ActOrc(moves[i].From, moves[i].To); err == nil {
				t.Fatalf("Illegal move %d", i)
			} else if !errors.Is(err, ErrMustCapture) {
				t.Fatalf("Wrong reason for move %d: %v", i, err)
			}
		}
	}

	if err := game.ActOrc(moves[3].From, moves[3].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_KingCaptureTopRight(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{2, 4}: Piece{King: true},
			},
			Humans: map[Coord]Piece{
				Coord{4, 6}: Piece{},
				Coord{0, 2}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{2, 4},
				To:   Coord{3, 5},
			},
			{
				From:     Coord{2, 4},
				To:       Coord{5, 7},
				Captured: Coord{4, 6},
			},
			{
				From: Coord{2, 4},
				To:   Coord{1, 5},
			},
			{
				From: Coord{2, 4},
				To:   Coord{0, 6},
			},
			{
				From: Coord{2, 4},
				To:   Coord{3, 3},
			},
			{
				From: Coord{2, 4},
				To:   Coord{4, 2},
			},
			{
				From: Coord{2, 4},
				To:   Coord{5, 1},
			},
			{
				From: Coord{2, 4},
				To:   Coord{6, 0},
			},
			{
				From: Coord{2, 4},
				To:   Coord{1, 3},
			},
		},
	)

	for i := 0; i < len(moves); i++ {
		if moves[i].Captured == (Coord{}) {
			if err := game.ActOrc(moves[i].From, moves[i].To); err == nil {
				t.Fatalf("Illegal move %d", i)
			} else if !errors.Is(err, ErrMustCapture) {
				t.Fatalf("Wrong reason for move %d: %v", i, err)
			}
		}
	}

	if err := game.ActOrc(moves[1].From, moves[1].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}

func TestOrc_KingNoCapture(t *testing.T) {
	game := State{
		Board: Board{
			Orcs: map[Coord]Piece{
				Coord{2, 4}: Piece{King: true},
			},
			Humans: map[Coord]Piece{
				Coord{0, 2}: Piece{},
			},
		},
		Current: Orc,
	}

	moves := slices.Collect(game.OrcMoves())

	if len(moves) != 10 {
		t.Fatalf("Wrong number of moves: %d", len(moves))
	}

	assertMoves(
		t,
		moves,
		[]Move{
			{
				From: Coord{2, 4},
				To:   Coord{3, 5},
			},
			{
				From: Coord{2, 4},
				To:   Coord{4, 6},
			},
			{
				From: Coord{2, 4},
				To:   Coord{5, 7},
			},
			{
				From: Coord{2, 4},
				To:   Coord{1, 5},
			},
			{
				From: Coord{2, 4},
				To:   Coord{0, 6},
			},
			{
				From: Coord{2, 4},
				To:   Coord{3, 3},
			},
			{
				From: Coord{2, 4},
				To:   Coord{4, 2},
			},
			{
				From: Coord{2, 4},
				To:   Coord{5, 1},
			},
			{
				From: Coord{2, 4},
				To:   Coord{6, 0},
			},
			{
				From: Coord{2, 4},
				To:   Coord{1, 3},
			},
		},
	)

	if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
}
