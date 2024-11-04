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

package front

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/checkers"
	"github.com/dimkr/tootik/front/text"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) checkers(w text.Writer, r *Request, args ...string) {
	pending, err := h.DB.QueryContext(
		r.Context,
		`
		select checkers.rowid, humans.actor, checkers.inserted from checkers
		join persons humans on
			humans.id = checkers.human
		where
			checkers.ended is null and
			checkers.orc is null
		order by
			checkers.inserted
		`,
	)
	if err != nil {
		r.Log.Error("Failed to fetch pending games", "error", err)
		w.Error()
		return
	}
	defer pending.Close()

	active, err := h.DB.QueryContext(
		r.Context,
		`
		select checkers.rowid, humans.actor, orcs.actor, coalesce(json_array_length(checkers.state->'$.turns'), 0) as turns, coalesce(checkers.updated, checkers.inserted) as ts from checkers
		join persons humans on
			humans.id = checkers.human
		join persons orcs on
			orcs.id = checkers.orc
		where
			checkers.ended is null and
			checkers.orc is not null
		order by
			ts desc
		`,
	)
	if err != nil {
		r.Log.Error("Failed to fetch active games", "error", err)
		w.Error()
		return
	}
	defer active.Close()

	ended, err := h.DB.QueryContext(
		r.Context,
		`
		select checkers.rowid, humans.actor, orcs.actor, checkers.ended from checkers
		join persons humans on
			humans.id = checkers.human
		join persons orcs on
			orcs.id = checkers.orc
		where
			checkers.ended is not null
		order by
			checkers.ended desc
		`,
	)
	if err != nil {
		r.Log.Error("Failed to fetch ended games", "error", err)
		w.Error()
		return
	}
	defer ended.Close()

	w.OK()
	w.Title("👑 Checkers")

	w.Subtitle("Pending Games")

	anyPending := false
	for pending.Next() {
		var human ap.Actor
		var rowID, inserted int64
		if err := pending.Scan(&rowID, &human, &inserted); err != nil {
			r.Log.Error("Failed to fetch active game", "error", err)
			continue
		}

		if r.User != nil {
			w.Linkf(fmt.Sprintf("/users/checkers/%d", rowID), "%s Game #%d: 🤺 %s", time.Unix(inserted, 0).Format(time.DateOnly), rowID, human.PreferredUsername)
		} else {
			w.Linkf(fmt.Sprintf("/checkers/%d", rowID), "%s Game #%d: 🤺 %s", time.Unix(inserted, 0).Format(time.DateOnly), rowID, human.PreferredUsername)
		}

		anyPending = true
	}

	if !anyPending {
		w.Text("No pending games.")
	}

	w.Empty()
	w.Subtitle("Active Games")

	anyActive := false
	for active.Next() {
		var human, orc ap.Actor
		var rowID, updated, turns int64
		if err := active.Scan(&rowID, &human, &orc, &turns, &updated); err != nil {
			r.Log.Error("Failed to fetch active game", "error", err)
			continue
		}

		if r.User != nil {
			w.Linkf(fmt.Sprintf("/users/checkers/%d", rowID), "%s Game #%d: 🤺 %s vs 🧌 %s: turn %d", time.Unix(updated, 0).Format(time.DateOnly), rowID, human.PreferredUsername, orc.PreferredUsername, turns)
		} else {
			w.Linkf(fmt.Sprintf("/checkers/%d", rowID), "%s Game #%d: 🤺 %s vs 🧌 %s: turn %d", time.Unix(updated, 0).Format(time.DateOnly), rowID, human.PreferredUsername, orc.PreferredUsername, turns)
		}

		anyActive = true
	}

	if !anyActive {
		w.Text("No active games.")
	}

	w.Empty()
	w.Subtitle("Ended Games")

	anyEnded := false
	for ended.Next() {
		var human, orc ap.Actor
		var rowID, inserted int64
		if err := ended.Scan(&rowID, &human, &orc, &inserted); err != nil {
			r.Log.Error("Failed to fetch ended game", "error", err)
			continue
		}

		if r.User != nil {
			w.Linkf(fmt.Sprintf("/users/checkers/%d", rowID), "%s Game #%d: 🤺 %s vs 🧌 %s", time.Unix(inserted, 0).Format(time.DateOnly), rowID, human.PreferredUsername, orc.PreferredUsername)
		} else {
			w.Linkf(fmt.Sprintf("/checkers/%d", rowID), "%s Game #%d: 🤺 %s vs 🧌 %s", time.Unix(inserted, 0).Format(time.DateOnly), rowID, human.PreferredUsername, orc.PreferredUsername)
		}

		anyEnded = true
	}

	if !anyEnded {
		w.Text("No ended games.")
	}

	if r.User != nil {
		w.Separator()
		w.Link("/users/checkers/start", "🤺 Start game")
	}
}

func (h *Handler) checkersStart(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	var playing int
	if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from checkers where (human = $1 or orc = $1) and ended is null)`, r.User.ID).Scan(&playing); err != nil {
		r.Log.Warn("Failed to check if already playing", "error", err)
		w.Error()
		return
	} else if playing == 1 {
		r.Log.Warn("User is already playing")
		w.Status(40, "Already playing another game")
		return
	}

	var ended sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context, `select max((select coalesce(max(ended), 0) from checkers where human = $1), (select coalesce(max(ended), 0) from checkers where orc = $1))`, r.User.ID).Scan(&ended); err != nil {
		r.Log.Warn("Failed to check last game end time", "error", err)
		w.Error()
		return
	} else if ended.Valid {
		t := time.Unix(ended.Int64, 0)
		can := t.Add(h.Config.MinCheckersInterval)
		until := time.Until(can)
		if until > 0 {
			r.Log.Warn("User is playing too frequently", "last", t, "can", can)
			w.Statusf(40, "Please wait for %s", until.Truncate(time.Second).String())
			return
		}
	}

	first := checkers.Human
	if h.Config.CheckersRandomizePlayer != nil && *h.Config.CheckersRandomizePlayer && rand.IntN(2) == 1 {
		first = checkers.Orc
	}

	res, err := h.DB.ExecContext(r.Context, `insert into checkers(human, state) values(?, ?)`, r.User.ID, checkers.Start(first))
	if err != nil {
		r.Log.Warn("Failed to insert game", "error", err)
		w.Error()
		return
	}

	rowID, err := res.LastInsertId()
	if err != nil {
		r.Log.Warn("Failed to insert game", "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/checkers/%d", rowID)
}

func (h *Handler) checkersJoin(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rowID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse row ID", "row_id", args[1], "error", err)
		w.Error()
		return
	}

	var playing sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context, `select rowid from checkers where (human = $1 or orc = $1) and ended is null`, r.User.ID).Scan(&playing); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to check if already playing", "row_id", rowID, "error", err)
		w.Error()
		return
	} else if err == nil && playing.Valid && playing.Int64 == rowID {
		r.Log.Warn("User already joined", "row_id", rowID)
		w.Status(40, "Already joined")
		return
	} else if err == nil && playing.Valid {
		r.Log.Warn("User is already playing", "row_id", rowID)
		w.Status(40, "Already playing another game")
		return
	}

	var ended sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context, `select max((select coalesce(max(ended), 0) from checkers where human = $1), (select coalesce(max(ended), 0) from checkers where orc = $1))`, r.User.ID).Scan(&ended); err != nil {
		r.Log.Warn("Failed to check last game end time", "row_id", rowID, "error", err)
		w.Error()
		return
	} else if ended.Valid {
		t := time.Unix(ended.Int64, 0)
		can := t.Add(h.Config.MinCheckersInterval)
		until := time.Until(can)
		if until > 0 {
			r.Log.Warn("User is playing too frequently", "last", t, "can", can)
			w.Statusf(40, "Please wait for %s", until.Truncate(time.Second).String())
			return
		}
	}

	var can int
	if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from checkers where orc is null and human != $1)`, r.User.ID).Scan(&can); err != nil {
		r.Log.Warn("Failed to join game", "row_id", rowID, "error", err)
		w.Error()
		return
	}
	if can == 0 {
		r.Log.Warn("Cannot join game", "row_id", rowID)
		w.Status(40, "Cannot join game")
		return
	}

	if _, err := h.DB.ExecContext(r.Context, `update checkers set orc = ?, updated = unixepoch() where rowid = ? and orc is null`, r.User.ID, rowID); err != nil {
		r.Log.Warn("Failed to join game", "row_id", rowID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/checkers/" + args[1])
}

func (h *Handler) checkersSurrender(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rowID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse row ID", "row_id", args[1], "error", err)
		w.Error()
		return
	}

	var playing int
	if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from checkers where (human = $1 or orc = $1) and ended is null)`, r.User.ID).Scan(&playing); err != nil {
		r.Log.Warn("Failed to check if already playing", "row_id", rowID, "error", err)
		w.Error()
		return
	}
	if playing == 0 {
		r.Log.Warn("User is not playing", "row_id", rowID)
		w.Status(40, "Not playing this game")
		return
	}

	if _, err := h.DB.ExecContext(r.Context, `update checkers set winner = (case when human = $1 then orc when orc is not null then human else null end), ended = unixepoch() where rowid = $2 and ended is null`, r.User.ID, rowID); err != nil {
		r.Log.Warn("Failed to surrender", "row_id", rowID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/checkers/" + args[1])
}

func (h *Handler) checkersView(w text.Writer, r *Request, args ...string) {
	rowID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse row ID", "row_id", args[1], "error", err)
		w.Error()
		return
	}

	var human ap.Actor
	var orc sql.Null[ap.Actor]
	var state checkers.State
	var winner sql.NullString
	var ended sql.NullInt64
	if err := h.DB.QueryRowContext(
		r.Context,
		`
		select humans.actor, orcs.actor, checkers.state, checkers.winner, checkers.ended from checkers
		join persons humans on
			humans.id = checkers.human
		left join persons orcs on
			orcs.id = checkers.orc
		where
			checkers.rowid = ?
		`,
		rowID,
	).Scan(&human, &orc, &state, &winner, &ended); err != nil {
		r.Log.Warn("Failed to fetch game", "row_id", rowID, "error", err)
		w.Error()
		return
	}

	w.OK()
	w.Titlef("👑 Game #%d: Turn %d", rowID, len(state.Turns))

	if r.User != nil && r.User.ID == human.ID {
		w.Link("/users/outbox/"+strings.TrimPrefix(human.ID, "https://"), "🤺 You")
	} else if r.User != nil && r.User.ID != human.ID {
		w.Linkf("/users/outbox/"+strings.TrimPrefix(human.ID, "https://"), "🤺 %s", human.PreferredUsername)
	} else if r.User == nil {
		w.Linkf("/outbox/"+strings.TrimPrefix(human.ID, "https://"), "🤺 %s", human.PreferredUsername)
	}

	if orc.Valid && r.User != nil && r.User.ID == orc.V.ID {
		w.Link("/users/outbox/"+strings.TrimPrefix(orc.V.ID, "https://"), "🧌 You")
	} else if orc.Valid && r.User != nil && r.User.ID != orc.V.ID {
		w.Linkf("/users/outbox/"+strings.TrimPrefix(orc.V.ID, "https://"), "🧌 %s", orc.V.PreferredUsername)
	} else if orc.Valid && r.User == nil {
		w.Linkf("/outbox/"+strings.TrimPrefix(orc.V.ID, "https://"), "🧌 %s", orc.V.PreferredUsername)
	}

	if len(state.Turns) > 1 {
		for i, turn := range state.Turns[1:] {
			w.Empty()
			w.Subtitlef("%s Turn %d", turn.Moved.Format(time.DateOnly), i)
			w.Raw("Board", turn.String())
		}

		w.Empty()
		w.Subtitlef("%s Turn %d", state.Moved.Format(time.DateOnly), len(state.Turns)-1)
	} else {
		w.Empty()
	}

	w.Raw("Board", state.String())

	if winner.Valid {
		w.Empty()

		if r.User != nil && winner.String == r.User.ID {
			w.Text("You won.")
		} else if winner.String == orc.V.ID {
			w.Textf("%s won.", orc.V.PreferredUsername)
		} else {
			w.Textf("%s won.", human.PreferredUsername)
		}

		return
	} else if ended.Valid {
		w.Empty()
		w.Textf("Game has ended at %s.", time.Unix(ended.Int64, 0).Format(time.DateOnly))
		return
	}

	if r.User != nil && !orc.Valid && human.ID != r.User.ID {
		w.Empty()
		w.Linkf(fmt.Sprintf("/users/checkers/join/%d", rowID), "🧌 Play orcs")
	} else if !orc.Valid {
		w.Empty()
		w.Text("Waiting for a player to join.")
	} else if !ended.Valid && (r.User == nil || r.User.ID == human.ID) && state.Current == checkers.Orc {
		w.Empty()
		w.Textf("It's %s's turn.", orc.V.PreferredUsername)
	} else if r.User != nil && human.ID == r.User.ID {
		w.Empty()

		moves := slices.SortedFunc(state.HumanMoves(), func(a, b checkers.Move) int {
			if a.Captured != (checkers.Coord{}) && b.Captured == (checkers.Coord{}) {
				return -1
			} else if a.Captured == (checkers.Coord{}) && b.Captured != (checkers.Coord{}) {
				return 1
			}

			if a.To.Y > b.To.Y && !state.Humans[a.From].King {
				return 1
			} else if a.To.Y < b.To.Y && !state.Humans[b.From].King {
				return -1
			}

			if a.From.Y > b.From.Y {
				return 1
			} else if a.From.Y < b.From.Y {
				return -1
			}

			if a.From.X > b.From.X {
				return 1
			} else if a.From.X < b.From.X {
				return -1
			}

			return 0
		})

		for _, move := range moves {
			if move.Captured == (checkers.Coord{}) {
				w.Linkf(fmt.Sprintf("/users/checkers/move/%d/%d%d%d%d", rowID, move.From.X, move.From.Y, move.To.X, move.To.Y), "Move human %d from %d,%d to %d,%d", state.Humans[move.From].ID, move.From.X, move.From.Y, move.To.X, move.To.Y)
			} else {
				w.Linkf(fmt.Sprintf("/users/checkers/move/%d/%d%d%d%d", rowID, move.From.X, move.From.Y, move.To.X, move.To.Y), "Move human %d from %d,%d to %d,%d (capture orc %d)", state.Humans[move.From].ID, move.From.X, move.From.Y, move.To.X, move.To.Y, state.Orcs[move.Captured].ID)
			}
		}
	} else if !ended.Valid && (r.User == nil || (orc.Valid && r.User.ID == orc.V.ID)) && state.Current == checkers.Human {
		w.Empty()
		w.Textf("It's %s's turn.", human.PreferredUsername)
	} else if r.User != nil && orc.Valid && orc.V.ID == r.User.ID {
		w.Empty()

		moves := slices.SortedFunc(state.OrcMoves(), func(a, b checkers.Move) int {
			if a.Captured != (checkers.Coord{}) && b.Captured == (checkers.Coord{}) {
				return -1
			} else if a.Captured == (checkers.Coord{}) && b.Captured != (checkers.Coord{}) {
				return 1
			}

			if a.To.Y > b.To.Y && !state.Orcs[a.From].King {
				return -1
			} else if a.To.Y < b.To.Y && !state.Orcs[b.From].King {
				return 1
			}

			if a.From.Y > b.From.Y {
				return -1
			} else if a.From.Y < b.From.Y {
				return 1
			}

			if a.From.X > b.From.X {
				return -1
			} else if a.From.X < b.From.X {
				return 1
			}

			return 0
		})

		for _, move := range moves {
			if move.Captured == (checkers.Coord{}) {
				w.Linkf(fmt.Sprintf("/users/checkers/move/%d/%d%d%d%d", rowID, move.From.X, move.From.Y, move.To.X, move.To.Y), "Move orc %d from %d,%d to %d,%d", state.Orcs[move.From].ID, move.From.X, move.From.Y, move.To.X, move.To.Y)
			} else {
				w.Linkf(fmt.Sprintf("/users/checkers/move/%d/%d%d%d%d", rowID, move.From.X, move.From.Y, move.To.X, move.To.Y), "Move orc %d from %d,%d to %d,%d (capture human %d)", state.Orcs[move.From].ID, move.From.X, move.From.Y, move.To.X, move.To.Y, state.Humans[move.Captured].ID)
			}
		}
	}

	if r.User != nil && (human.ID == r.User.ID || (orc.Valid && orc.V.ID == r.User.ID)) {
		w.Empty()
		w.Link(fmt.Sprintf("/users/checkers/surrender/%d", rowID), "🏃 Surrender")
	}
}

func (h *Handler) checkersMove(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rowID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse row ID", "row_id", args[1], "error", err)
		w.Error()
		return
	}

	fromX, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse from X", "x", args[2], "error", err)
		w.Error()
		return
	}

	fromY, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse from Y", "y", args[3], "error", err)
		w.Error()
		return
	}

	toX, err := strconv.ParseInt(args[4], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse to X", "x", args[4], "error", err)
		w.Error()
		return
	}

	toY, err := strconv.ParseInt(args[5], 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse to Y", "y", args[5], "error", err)
		w.Error()
		return
	}

	var human, orc ap.Actor
	var state checkers.State
	if err := h.DB.QueryRowContext(
		r.Context,
		`
		select humans.actor, orcs.actor, checkers.state from checkers
		join persons humans on
			humans.id = checkers.human
		join persons orcs on
			orcs.id = checkers.orc
		where
			checkers.rowid = ? and
			checkers.ended is null
		`,
		rowID,
	).Scan(&human, &orc, &state); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("No such game", "row_id", rowID)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch game", "row_id", rowID, "error", err)
		w.Error()
		return
	}

	act := state.ActHuman
	moves := state.OrcMoves
	if orc.ID == r.User.ID {
		act = state.ActOrc
		moves = state.HumanMoves

		if state.Current != checkers.Orc {
			w.Status(40, "Wait for your turn")
			return
		}
	} else if human.ID != r.User.ID {
		r.Log.Warn("Game belongs to another player", "row_id", rowID, "human", human.ID, "orc", orc.ID)
		w.Error()
		return
	} else if state.Current != checkers.Human {
		w.Status(40, "Wait for your turn")
		return
	}

	if err := act(checkers.Coord{X: int(fromX), Y: int(fromY)}, checkers.Coord{X: int(toX), Y: int(toY)}); err != nil {
		if errors.Is(err, checkers.ErrMustCapture) {
			w.Status(40, "Must capture")
			return
		}

		if errors.Is(err, checkers.ErrInvalidMove) {
			w.Status(40, "Invalid move")
			return
		}

		r.Log.Warn("Failed to act", "row_id", rowID, "human", human.ID, "orc", orc.ID, "error", err)
		w.Error()
		return
	}

	won := true
	for _ = range moves() {
		won = false
		break
	}

	if won {
		if _, err := h.DB.ExecContext(r.Context, `update checkers set state = ?, winner = ?, updated = unixepoch(), ended = unixepoch() where rowid = ?`, &state, r.User.ID, rowID); err != nil {
			r.Log.Warn("Failed to act", "row_id", rowID, "human", human.ID, "orc", orc.ID, "error", err)
			w.Error()
			return
		}
	} else if _, err := h.DB.ExecContext(r.Context, `update checkers set state = ?, updated = unixepoch() where rowid = ?`, &state, rowID); err != nil {
		r.Log.Warn("Failed to act", "row_id", rowID, "human", human.ID, "orc", orc.ID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/checkers/" + args[1])
}
