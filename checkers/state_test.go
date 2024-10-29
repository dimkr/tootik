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
	"math/rand/v2"
	"slices"
	"testing"
)

func TestState_HappyFlow(t *testing.T) {
	first := Human
	if rand.Int()%2 == 1 {
		first = Orc
	}

	game := Start(first)

	for {
		if game.Current == Orc {
			moves := slices.SortedFunc(game.OrcMoves(), func(a, b Move) int {
				if a.Captured != (Coord{}) && b.Captured == (Coord{}) {
					return -1
				}

				return rand.Int() % 2
			})

			if len(moves) == 0 {
				break
			}

			if err := game.ActOrc(moves[0].From, moves[0].To); err != nil {
				t.Fatalf("Move failed: %v", err)
			}
		} else {
			moves := slices.SortedFunc(game.HumanMoves(), func(a, b Move) int {
				if a.Captured != (Coord{}) && b.Captured == (Coord{}) {
					return -1
				}

				return rand.Int() % 2
			})

			if len(moves) == 0 {
				break
			}

			if err := game.ActHuman(moves[0].From, moves[0].To); err != nil {
				t.Fatalf("Move failed: %v", err)
			}
		}
	}
}
