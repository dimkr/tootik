/*
Copyright 2026 Dima Krasner

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

package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/proof"
)

func (q *Queue) backfill(ctx context.Context, activity *ap.Activity) error {
	if !(activity.Type == ap.Create || activity.Type == ap.Update) {
		return nil
	}

	post, ok := activity.Object.(*ap.Object)
	if !ok {
		return nil
	}

	return q.fetchParent(ctx, post, 0)
}

func (q *Queue) fetchParent(ctx context.Context, post *ap.Object, depth int) error {
	if depth == q.Config.BackfillDepth {
		return errors.New("reached backfill depth")
	}

	if !(post.Type == ap.Note || post.Type == ap.Page || post.Type == ap.Article || post.Type == ap.Question) {
		return nil
	}

	if !post.IsPublic() {
		return nil
	}

	parentOrigin, err := ap.Origin(post.InReplyTo)
	if err != nil {
		return err
	}

	since := time.Now().Add(-q.Config.BackfillInterval)

	var parent ap.Object
	if err := q.DB.QueryRowContext(
		ctx,
		`
		select json(object) from notes
		where
			id = $1
			and (
				deleted = 1
				or exists (
					select 1 from persons where
						persons.id = notes.author
						and persons.ed25519privkey is not null
				)
				or (
					not exists (
						select 1 from persons where
							persons.id = notes.author
							and persons.ed25519privkey is not null
					) and (
						max(inserted, updated) > $2
						or exists (
							select 1 from history where
								(activity->>'$.type' = 'Create' or activity->>'$.type' = 'Update')
								and activity->>'$.object.id' = $1
								and inserted > $3
						)
					)
				)
			)
		`,
		post.InReplyTo,
		since.Unix(),
		since.UnixNano(),
	).Scan(&parent); err == nil {
		slog.Debug("Skipping fetching of parent post", "parent", post.InReplyTo, "depth", depth)
		return q.fetchParent(ctx, &parent, depth+1)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if post.InReplyTo == "" {
		slog.Debug("Reached end of thread", "post", post.ID, "depth", depth)
		return nil
	}

	slog.Info("Backfilling thread", "post", post.ID, "parent", post.InReplyTo, "depth", depth)

	resp, err := q.Resolver.Get(ctx, q.Keys, post.InReplyTo)
	if err != nil && resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
		slog.Info("Deleting backfilled parent post", "parent", post.InReplyTo)

		tx, err := q.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if err := q.Inbox.ProcessActivity(
			ctx,
			tx,
			sql.NullString{},
			&ap.Actor{},
			&ap.Activity{
				ID:   post.InReplyTo,
				Type: ap.Delete,
				Object: &ap.Object{
					ID: post.InReplyTo,
				},
			},
			"",
			0,
			false,
		); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		slog.Info("Deleted backfilled parent post", "parent", post.InReplyTo)
		return nil
	} else if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.ContentLength > q.Config.MaxResponseBodySize {
		return errors.New("response is too big")
	}

	var body []byte
	if resp.ContentLength >= 0 {
		body = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, body)
	} else {
		body, err = io.ReadAll(io.LimitReader(resp.Body, q.Config.MaxResponseBodySize))
	}
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &parent); err != nil {
		return err
	}

	if parent.ID != post.InReplyTo {
		return fmt.Errorf("%s is not %s", parent.ID, post.InReplyTo)
	}

	if !(parent.Type == ap.Note || parent.Type == ap.Page || parent.Type == ap.Article || parent.Type == ap.Question) {
		return nil
	}

	update := &ap.Activity{
		ID:     parent.ID,
		Type:   ap.Update,
		Actor:  parent.AttributedTo,
		Object: &parent,
	}

	if err := ap.ValidateOrigin(q.Domain, update, parentOrigin); err != nil {
		return err
	}

	if ap.IsPortable(parent.ID) {
		m := ap.KeyRegex.FindStringSubmatch(parent.Proof.VerificationMethod)
		if m == nil {
			return fmt.Errorf("%s does not contain a public key", parent.Proof.VerificationMethod)
		}

		if suffix, ok := strings.CutPrefix(parentOrigin, "did:key:"); !ok || suffix != m[1] {
			return fmt.Errorf("key %s does not belong to %s", m[1], parentOrigin)
		}

		publicKey, err := data.DecodeEd25519PublicKey(m[1])
		if err != nil {
			return fmt.Errorf("failed to verify proof using %s: %w", parent.Proof.VerificationMethod, err)
		}

		if err := proof.Verify(publicKey, parent.Proof, body); err != nil {
			return err
		}
	}

	parentAuthor, err := q.Resolver.ResolveID(ctx, q.Keys, parent.AttributedTo, 0)
	if err != nil {
		return err
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := q.Inbox.ProcessActivity(
		ctx,
		tx,
		sql.NullString{},
		parentAuthor,
		update,
		"",
		0,
		false,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	slog.Info("Backfilled thread", "post", post.ID, "parent", parent.ID, "depth", depth)

	return q.fetchParent(ctx, &parent, depth+1)
}
