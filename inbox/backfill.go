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

func (q *Queue) fetchPost(ctx context.Context, id string) (*ap.Object, error) {
	postOrigin, err := ap.Origin(id)
	if err != nil {
		return nil, err
	}

	var post ap.Object
	if err := q.DB.QueryRowContext(
		ctx,
		/*
			we want to use the post we have and avoid fetching if
			1. it was deleted, or
			2. it's a post by a local user, or
			3. it's *not* a post by a local user, but it was updated recently or it's likely that edits and deletion
			   will be federated to us because we've received at least one activity
		*/
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
								and coalesce(activity->>'$.actor.id', activity->>'$.actor') = notes.author
								and activity->>'$.object.id' = $1
						)
					)
				)
			)
		`,
		id,
		time.Now().Add(-q.Config.BackfillInterval).Unix(),
	).Scan(&post); err == nil {
		slog.Debug("Skipping fetching of post", "id", id)
		return &post, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	slog.Info("Fetching post", "post", id)

	resp, err := q.Resolver.Get(ctx, q.Keys, id)
	if err != nil && resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
		slog.Info("Deleting backfilled parent post", "post", id)

		tx, err := q.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		if err := q.Inbox.ProcessActivity(
			ctx,
			tx,
			sql.NullString{},
			&ap.Actor{},
			&ap.Activity{
				ID:   id,
				Type: ap.Delete,
				Object: &ap.Object{
					ID: id,
				},
			},
			"",
			0,
			false,
		); err != nil {
			return nil, err
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}

		slog.Info("Deleted backfilled post", "id", id)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.ContentLength > q.Config.MaxResponseBodySize {
		return nil, errors.New("response is too big")
	}

	var body []byte
	if resp.ContentLength >= 0 {
		body = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, body)
	} else {
		body, err = io.ReadAll(io.LimitReader(resp.Body, q.Config.MaxResponseBodySize))
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &post); err != nil {
		return nil, err
	}

	if post.ID != id {
		return nil, fmt.Errorf("%s is not %s", post.ID, id)
	}

	if !(post.Type == ap.Note || post.Type == ap.Page || post.Type == ap.Article || post.Type == ap.Question) {
		return &post, nil
	}

	update := &ap.Activity{
		ID:     post.ID,
		Type:   ap.Update,
		Actor:  post.AttributedTo,
		Object: &post,
	}

	if err := ap.ValidateOrigin(q.Domain, update, postOrigin); err != nil {
		return nil, err
	}

	if ap.IsPortable(post.ID) {
		m := ap.KeyRegex.FindStringSubmatch(post.Proof.VerificationMethod)
		if m == nil {
			return nil, fmt.Errorf("%s does not contain a public key", post.Proof.VerificationMethod)
		}

		if suffix, ok := strings.CutPrefix(postOrigin, "did:key:"); !ok || suffix != m[1] {
			return nil, fmt.Errorf("key %s does not belong to %s", m[1], postOrigin)
		}

		publicKey, err := data.DecodeEd25519PublicKey(m[1])
		if err != nil {
			return nil, fmt.Errorf("failed to verify proof using %s: %w", post.Proof.VerificationMethod, err)
		}

		if err := proof.Verify(publicKey, post.Proof, body); err != nil {
			return nil, err
		}
	}

	parentAuthor, err := q.Resolver.ResolveID(ctx, q.Keys, post.AttributedTo, 0)
	if err != nil {
		return nil, err
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	slog.Info("Backfilled post", "post", id)
	return &post, nil
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

	if post.InReplyTo == "" {
		slog.Debug("Reached end of thread", "post", post.ID, "depth", depth)
		return q.fetchContext(ctx, post)
	}

	parent, err := q.fetchPost(ctx, post.InReplyTo)
	if err != nil {
		return err
	}

	return q.fetchParent(ctx, parent, depth+1)
}

func (q *Queue) fetchContext(ctx context.Context, post *ap.Object) error {
	if post.BackfillContext == "" {
		return nil
	}

	slog.Info("Fetching context", "server", q.Domain, "context", post.BackfillContext)

	postOrigin, err := ap.Origin(post.ID)
	if err != nil {
		return fmt.Errorf("failed to determine origin of %s: %w", post.ID, err)
	}

	contextOrigin, err := ap.Origin(post.BackfillContext)
	if err != nil {
		return fmt.Errorf("failed to determine origin of %s: %w", post.BackfillContext, err)
	}

	if contextOrigin != postOrigin {
		return fmt.Errorf("%s does not belong to %s", postOrigin, contextOrigin)
	}

	resp, err := q.Resolver.Get(ctx, q.Keys, post.BackfillContext)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", post.BackfillContext, err)
	}
	defer resp.Body.Close()

	if resp.ContentLength > q.Config.MaxResponseBodySize {
		return errors.New(post.BackfillContext + " is too big")
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

	var collection ap.Collection
	if err := json.Unmarshal(body, &collection); err != nil {
		return err
	}

	if collection.ID != post.BackfillContext {
		return fmt.Errorf("%s is not %s", collection.ID, post.BackfillContext)
	}

	if collection.AttributedTo != post.AttributedTo {
		return fmt.Errorf("%s is not owned by %s", collection.AttributedTo, post.AttributedTo)
	}

	if collection.First == nil {
		return errors.New("no first page in " + post.BackfillContext)
	}

	m, ok := collection.First.(map[string]any)
	if !ok {
		return errors.New("invalid first page in " + post.BackfillContext)
	}

	items := m["items"]
	if items == nil {
		return errors.New("no items in " + post.BackfillContext)
	}

	l, ok := items.([]any)
	if !ok {
		return errors.New("invalid items in " + post.BackfillContext)
	}

	for i := len(l) - 1; i > 0; i-- {
		s, ok := l[i].(string)
		if !ok {
			return errors.New("non-string in " + post.BackfillContext)
		}

		if s == post.ID {
			continue
		}

		var depth sql.NullInt64
		if err := q.DB.QueryRowContext(
			ctx,
			`
			select max(depth) from (
				with recursive thread(id, depth, parent) as (
					select notes.id, 1 as depth, notes.object->>'$.inReplyTo' as parent from notes
					where notes.id = $1
					union all
					select notes.id, t.depth + 1, notes.object->>'$.inReplyTo' as parent from thread t
					join notes on notes.id = t.parent
					where notes.public = 1
				)
				select depth from thread
			)
			`,
			s,
		).Scan(&depth); err != nil {
			return fmt.Errorf("failed to check depth of %s: %w", s, err)
		}

		if depth.Valid && depth.Int64 > int64(q.Config.BackfillDepth) {
			slog.Info("Skipping on depth", "s", s, "depth", depth)
			continue
		}

		slog.Info("Fetching on depth", "s", s, "depth", depth, "to", q.Domain, "max", q.Config.BackfillDepth)

		if _, err := q.fetchPost(ctx, s); err != nil {
			slog.Warn("Failed to fetch post", "id", s)
			continue
		}
	}

	return nil
}
