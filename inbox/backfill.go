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

func (q *Queue) fetchCachedPost(ctx context.Context, id string) (*ap.Object, error) {
	var post ap.Object
	return &post, q.DB.QueryRowContext(
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
	).Scan(&post)
}

func (q *Queue) backfill(ctx context.Context, activity *ap.Activity) error {
	if !(activity.Type == ap.Create || activity.Type == ap.Update) {
		return nil
	}

	post, ok := activity.Object.(*ap.Object)
	if !ok {
		return nil
	}

	fetchErr := q.fetchParent(ctx, post, 0)

	var contextErr error
	var headID string
	if err := q.DB.QueryRowContext(
		ctx,
		`
		select id from
		(
			with recursive thread(id, object, depth) as (
				select id, object, 0 as depth
				from notes
				where id = ?
				union all
				select notes.id, notes.object, t.depth + 1
				from thread t
				join notes on notes.id = t.object->>'$.inReplyTo'
			)
			select id, object, depth from thread order by depth desc
			limit 1
		)
		where object->>'$.inReplyTo' is null
		limit 1
		`,
		post.ID,
	).Scan(&headID); err == nil {
		if _, err := q.fetchCachedPost(ctx, headID); errors.Is(err, sql.ErrNoRows) {
			_, contextErr = q.fetchPost(ctx, headID)
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		contextErr = q.fetchContext(ctx, post)
	} else {
		contextErr = err
	}

	return errors.Join(fetchErr, contextErr)
}

func (q *Queue) fetchPost(ctx context.Context, id string) (*ap.Object, error) {
	origin, err := ap.Origin(id)
	if err != nil {
		return nil, err
	}

	resp, err := q.Resolver.Get(ctx, q.Keys, id)
	if err != nil && resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
		slog.Info("Deleting backfilled post", "id", id)

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

		return nil, errors.New(id + " is gone")
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

	var post ap.Object
	if err := json.Unmarshal(body, &post); err != nil {
		return nil, err
	}

	if post.ID != id {
		return nil, fmt.Errorf("%s is not %s", post.ID, id)
	}

	update := &ap.Activity{
		ID:     post.ID,
		Type:   ap.Update,
		Actor:  post.AttributedTo,
		Object: &post,
	}

	if err := ap.ValidateOrigin(q.Domain, update, origin); err != nil {
		return nil, err
	}

	if ap.IsPortable(post.ID) {
		m := ap.KeyRegex.FindStringSubmatch(post.Proof.VerificationMethod)
		if m == nil {
			return nil, fmt.Errorf("%s does not contain a public key", post.Proof.VerificationMethod)
		}

		if suffix, ok := strings.CutPrefix(origin, "did:key:"); !ok || suffix != m[1] {
			return nil, fmt.Errorf("key %s does not belong to %s", m[1], origin)
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

	return &post, nil
}

func (q *Queue) fetchParent(ctx context.Context, post *ap.Object, depth int) error {
	if depth == q.Config.BackfillDepth {
		return errors.New("reached backfill depth")
	}

	if post.InReplyTo == "" {
		return nil
	}

	if !post.IsPublic() {
		return nil
	}

	if cached, err := q.fetchCachedPost(ctx, post.InReplyTo); err == nil {
		slog.Debug("Skipping fetching of parent post", "parent", post.InReplyTo, "depth", depth)
		return q.fetchParent(ctx, cached, depth+1)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	parent, err := q.fetchPost(ctx, post.InReplyTo)
	if err != nil {
		return err
	}

	slog.Info("Backfilled thread", "post", post.ID, "parent", parent.ID, "depth", depth)

	return q.fetchParent(ctx, parent, depth+1)
}

func (q *Queue) fetchContext(ctx context.Context, post *ap.Object) error {
	if q.Config.BackfillDepth < 1 {
		return nil
	}

	if post.InReplyTo == "" {
		return nil
	}

	if post.BackfillContext == "" {
		return nil
	}

	contextOrigin, err := ap.Origin(post.BackfillContext)
	if err != nil {
		return fmt.Errorf("failed to determine origin of %s: %w", post.BackfillContext, err)
	}

	var exists int
	if err := q.DB.QueryRowContext(ctx, `select exists (select 1 from notes where id = ?)`, post.InReplyTo).Scan(&exists); err != nil {
		return err
	} else if exists == 1 {
		return nil
	}

	slog.Info("Fetching context", "server", q.Domain, "context", post.BackfillContext)

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

	if collection.First == nil {
		return errors.New("no first page in " + post.BackfillContext)
	}

	m, ok := collection.First.(map[string]any)
	if !ok {
		return errors.New("invalid first page in " + post.BackfillContext)
	}

	items := m["items"]
	if items == nil {
		return errors.New("no list of items in " + post.BackfillContext)
	}

	l, ok := items.([]any)
	if !ok {
		return errors.New("invalid items in " + post.BackfillContext)
	}

	if len(l) == 0 {
		return errors.New("empty list of items in " + post.BackfillContext)
	}

	s, ok := l[0].(string)
	if !ok {
		return errors.New("non-string in " + post.BackfillContext)
	}

	if s == post.ID {
		return nil
	}

	headOrigin, err := ap.Origin(s)
	if err != nil {
		return fmt.Errorf("failed to determine origin of %s: %w", post.ID, err)
	}

	if headOrigin != contextOrigin {
		return fmt.Errorf("%s does not belong to %s", headOrigin, contextOrigin)
	}

	/*
		fetch the toplevel post (the first item) if
		1. the topmost reply we have appears in the list of items
		2. its parent appears in the list, before it
		3. the first item is not the topmost reply or its parent
	*/
	var first string
	parentIndex := -1
	for i, item := range l {
		s, ok := item.(string)
		if !ok {
			if i == 0 {
				break
			}

			parentIndex = -1
			continue
		}

		if i == 0 {
			first = s
		}

		if s == post.InReplyTo {
			parentIndex = i
			continue
		}

		if s != post.ID {
			continue
		}

		if !(parentIndex >= 0 && i > parentIndex && first != "" && first != post.ID && first != post.InReplyTo) {
			return nil
		}

		if _, err := q.fetchCachedPost(ctx, first); err == nil {
			slog.Debug("Skipping fetching of thread head", "id", first)
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		_, err = q.fetchPost(ctx, first)
		return err
	}

	return nil
}
