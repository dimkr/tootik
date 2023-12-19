/*
Copyright 2023 Dima Krasner

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

package test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"strings"
	"testing"
)

func TestThread_TwoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Hi%%20Bob", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(strings.Split(view, "\n"), fmt.Sprintf("=> /users/view/%s View parent post", hash))
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+hash, server.Alice)
	assert.Contains(thread, "Replies to 😈 bob")
	assert.Contains(thread, " bob")
	assert.Contains(thread, " · alice")
	assert.Contains(thread, " · carol")
}

func TestThread_NestedReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Hi%%20Bob", reply[15:len(reply)-2]), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.NotContains(view, "View parent post")
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+hash, server.Alice)
	assert.Contains(thread, "Replies to 😈 bob")
	assert.Contains(thread, " bob")
	assert.Contains(thread, " · alice")
	assert.Contains(thread, " ·· carol")
}

func TestThread_NestedReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.NotContains(view, "View parent post")
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+hash, server.Alice)
	assert.Contains(thread, "Replies to 😈 bob")
	assert.Contains(thread, " bob")
	assert.Contains(thread, " · alice")
	assert.NotContains(thread, "carol")
}

func TestThread_NoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.NotContains(view, "View parent post")
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+hash, server.Alice)
	assert.Contains(thread, "Replies to 😈 bob")
	assert.Contains(thread, " bob")
	assert.NotContains(thread, "alice")
	assert.NotContains(thread, "carol")
}

func TestThread_NestedRepliesFromBottom(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	parentReplyHash := reply[15 : len(reply)-2]

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Hi%%20Bob", parentReplyHash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(strings.Split(view, "\n"), fmt.Sprintf("=> /users/view/%s View parent post", parentReplyHash))
	assert.Contains(strings.Split(view, "\n"), fmt.Sprintf("=> /users/view/%s View first post in thread", hash))
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(thread, "Replies to 😈 carol")
	assert.NotContains(thread, "· bob")
	assert.NotContains(thread, "alice")
	assert.Contains(thread, "carol")
}

func TestThread_NestedRepliesFromBottomMissingNode(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	firstReplyHash := reply[15 : len(reply)-2]

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Hi%%20Bob", firstReplyHash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	delete := server.Handle("/users/delete/"+firstReplyHash, server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), delete)

	view := server.Handle("/users/view/"+reply[15:len(reply)-2], server.Alice)
	assert.NotContains(view, "View parent post")
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(thread, "Replies to 😈 carol")
	assert.NotContains(thread, "bob")
	assert.NotContains(thread, "alice")
	assert.Contains(thread, "carol")
}

func TestThread_NestedRepliesFromBottomMissingFirstNode(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	parentReplyHash := reply[15 : len(reply)-2]

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Hi%%20Bob", parentReplyHash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), delete)

	view := server.Handle("/users/view/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(strings.Split(view, "\n"), fmt.Sprintf("=> /users/view/%s View parent post", parentReplyHash))
	assert.NotContains(view, "View first post in thread")
	assert.NotContains(view, "View thread")

	thread := server.Handle("/users/thread/"+reply[15:len(reply)-2], server.Alice)
	assert.Contains(thread, "Replies to 😈 carol")
	assert.NotContains(thread, "bob")
	assert.NotContains(thread, "alice")
	assert.Contains(thread, "carol")
}

func TestThread_Tree(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	to := ap.Audience{}
	to.Add(ap.Public)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/6",
				Type:         ap.NoteObject,
				AttributedTo: server.Carol.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/4",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.NoteObject,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/4",
				Type:         ap.NoteObject,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/2",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/2",
				Type:         ap.NoteObject,
				AttributedTo: server.Bob.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/3",
				Type:         ap.NoteObject,
				AttributedTo: server.Carol.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/5",
				Type:         ap.NoteObject,
				AttributedTo: server.Bob.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/3",
			},
		),
	)

	assert.NoError(tx.Commit())

	thread := server.Handle("/thread/f42d4ca96d006a7b924cde2b3694d2dacb0fe92e140232e336a0ad3cbc049984", nil)
	assert.Contains(thread, "Replies to 😈 alice")
	assert.Contains(thread, " · bob")
	assert.Contains(thread, " ·· alice")
	assert.Contains(thread, " ··· carol")
	assert.Contains(thread, " · carol")
	assert.Contains(thread, " ·· bob")
	assert.NotContains(strings.Split(thread, "\n"), "=> /view/f42d4ca96d006a7b924cde2b3694d2dacb0fe92e140232e336a0ad3cbc049984 View first post in thread")
}

func TestThread_SubTree(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	to := ap.Audience{}
	to.Add(ap.Public)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/6",
				Type:         ap.NoteObject,
				AttributedTo: server.Carol.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/4",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.NoteObject,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/4",
				Type:         ap.NoteObject,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/2",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/2",
				Type:         ap.NoteObject,
				AttributedTo: server.Bob.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/3",
				Type:         ap.NoteObject,
				AttributedTo: server.Carol.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/5",
				Type:         ap.NoteObject,
				AttributedTo: server.Bob.ID,
				Content:      "hello",
				To:           to,
				InReplyTo:    "https://localhost.localdomain:8443/note/3",
			},
		),
	)

	assert.NoError(tx.Commit())

	thread := server.Handle("/thread/6f5c6eb90e28bac6a71c73c93d8957e7d6627926838539561a918eaf21a4a800", nil)
	assert.Contains(thread, "Replies to 😈 bob")
	assert.Contains(thread, " bob")
	assert.Contains(thread, " · alice")
	assert.Contains(thread, " ·· carol")
	assert.Contains(strings.Split(thread, "\n"), "=> /view/f42d4ca96d006a7b924cde2b3694d2dacb0fe92e140232e336a0ad3cbc049984 View first post in thread")
}
