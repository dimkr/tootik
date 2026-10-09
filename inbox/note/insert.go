/*
Copyright 2023 - 2026 Dima Krasner

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

// Package note handles insertion of posts.
package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text/plain"
)

// Flatten converts a post into text that can be indexed for search purposes.
func Flatten(note *ap.Object) string {
	content, links := plain.FromHTML(note.Content)
	for _, mention := range note.Tag {
		if mention.Type == ap.Mention {
			links.Store(mention.Href, mention.Href)
		}
	}
	for _, attachment := range note.Attachment {
		if attachment.Href != "" {
			links.Store(attachment.Href, attachment.Href)
		} else if attachment.URL != "" {
			links.Store(attachment.URL, attachment.URL)
		}
	}
	var b strings.Builder
	b.WriteString(content)
	b.WriteByte(' ')
	b.WriteString(note.AttributedTo)
	for link, alt := range links.All() {
		if alt != "" {
			b.WriteString(alt)
			b.WriteByte(' ')
		}
		b.WriteString(link)
	}
	return b.String()
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CompatibleID returns an ID in compatible form: if it's an ap:// URI, it's converted to a URL of the first gateway of
// a cached actor with the same origin, or to a URL of the first location hint.
func CompatibleID(ctx context.Context, db querier, id string) (string, error) {
	parsed, err := ap.ParseID(id)
	if err != nil {
		return "", err
	}

	if parsed.Kind != ap.PortableID {
		return id, nil
	}

	var gw string
	if err := db.QueryRowContext(
		ctx,
		`select actor->>'$.gateways[0]' from persons where cid >= $1 and cid < $2 and actor->>'$.gateways[0]' is not null order by ed25519seed is not null desc, updated desc limit 1`,
		"ap://"+parsed.Origin+"/",
		"ap://"+parsed.Origin+"0",
	).Scan(&gw); errors.Is(err, sql.ErrNoRows) && len(parsed.Gateways) > 0 {
		gw = parsed.Gateways[0]
	} else if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no gateway for %s", id)
	} else if err != nil {
		return "", fmt.Errorf("failed to find gateway for %s: %w", id, err)
	}

	return ap.Gateway(gw, parsed.Canonical), nil
}

// Insert inserts a post.
func Insert(ctx context.Context, tx *sql.Tx, note *ap.Object) error {
	public := 0
	if note.IsPublic() {
		public = 1
	}

	id, err := CompatibleID(ctx, tx, note.ID)
	if err != nil {
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	author, err := CompatibleID(ctx, tx, note.AttributedTo)
	if err != nil {
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	// posts with ap:// IDs are linked using their canonical IDs
	slug := ap.Slug(id)
	if parsed, err := ap.ParseID(note.ID); err == nil && parsed.Kind == ap.PortableID {
		slug = ap.Slug(parsed.Canonical)
	}

	var pk int64
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO notes (slug, id, author, object, public) VALUES (?, ?, ?, JSONB(?), ?) RETURNING pk`,
		slug,
		id,
		author,
		&note,
		public,
	).Scan(&pk); err != nil {
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO notesfts (rowid, content) VALUES(?,?)`,
		pk,
		Flatten(note),
	); err != nil {
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	return nil
}
