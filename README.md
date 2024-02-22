```
          ..    .        ..                                        ..
 ...            .       ....                     ...
 ...   .     .   .   .   ... ..      .  .     .  . ...                 ...
 ..    .  .    ..   .         .    .. .       .. ..     .                   .
 .,     .               . . . ..             .  ..                  .        .
 .                .     . .. .. .... .       .  .  .               ..  ..
            .              ..   ...  .       .    .   .   .        ..  .   .
 .   .        .   ..        .    ..             ...' .. .          .   .    . .
 . .              . .    .  __     .     .__  _ __ ,; .'. .  .     ....
    . .          .         / /____  ___  /./_(_) /__  .'  ..         . .  . .
  ..      ... .    .  .   /.__/ _ \/ _ \/ __/./  '_/.   .       .. .   .    .
  .'   ...  .             \__/\___/\___/\__/_/_/\_\              .  .  . .
      .         .    .    . .    .    ...     ... .           .      ..
 ..  .. .   . .... ..  .  .         ..   .  .     .          .  .  ... ....' .
   ...  .      .   .  .. .  ... ...      . ..   ..         .,..    .....
 .   ..   ......             . .''.  .  ..          .         . .  ...
 ' .      .. ..  ..     . . ... ......::.   ..       .,.       .  .. ....    ..
 . ....  . .....     .  .. .  . ... . .,'.   .        ..          ,..  ..
 . .    .  .  . ..   .  .   .. .  .     ..     ..  .  . .       . . .        .'
   .  ....   '...                ...    . .  ..  .     ...     . '.   '     ...

# localhost.localdomain:8443

Welcome, fedinaut! localhost.localdomain:8443 is an instance of tootik, a federated nanoblogging service.

────

📻 My radio
📞 Mentions
⚡️ Followed users
😈 My profile
📡 This planet
✨ FOMO from outer space
🔥 Hashtags
🔭 Find user
🔎 Search posts
💌 Post to mentioned users
🔔 Post to followers
📣 Post to public
⚙️ Settings
📊 Statistics
🛟 Help
```

[![Latest release](https://img.shields.io/github/v/release/dimkr/tootik)](https://github.com/dimkr/tootik/releases) [![Build status](https://github.com/dimkr/tootik/actions/workflows/ci.yml/badge.svg)](https://github.com/dimkr/tootik/actions) [![Go Reference](https://pkg.go.dev/badge/github.com/dimkr/tootik.svg)](https://pkg.go.dev/github.com/dimkr/tootik)

## Overview

tootik is a federated nanoblogging service for the small internet. With tootik, you can interact with your friends, including those on [Mastodon](https://joinmastodon.org/), [Lemmy](https://join-lemmy.org/) and other [ActivityPub](https://www.w3.org/TR/activitypub/)-compatible servers, from the comfort of a minimalistic, text-based interface in the small internet:

```
                         Gemini           ActivityPub (HTTPS)
                           →                     ⇄
 ┌───────────────────────┐   ┌─────────────────┐   ┌─────────────────────────┐
 │  Bob's Gemini client  ├─┬─┤ tootik instance ├─┬─┤ Another tootik instance │
 ├───────────────────────┤ │ ├─────────────────┤ │ └─────────────────────────┘
 │2024-01-01 alice       │ │ │$ ./tootik ...   │ │ ┌────────────────┐
 │> Hi @bob and @carol!  │ │ └─────────────────┘ ├─┤ Something else │
 │...                    │ │                     │ └────────────────┘
 └───────────────────────┘ │                     │ ┌───────────────────┐
               ┌───────────┴───────────┐         └─┤ Mastodon instance ├─┐
               │ Alice's Gemini client │           └───────────────────┘ │
               ├───────────────────────┤              ┌──────────────────┴────┐
               │2024-01-01 bob         │              │  Carol's web browser  │
               │> Hi @alice!           │              ├───────────────────────┤
               │...                    │              │╔═╗ alice              │
               └───────────────────────┘              │╚═╝             17h ago│
                                                      │Hi @bob and @carol!    │
                                                      │                       │
                                                      │  ╔═╗ bob              │
                                                      │  ╚═╝           16h ago│
                                                      │  Hi @alice!           │
                                                      ├───────────────────────┤
                                                      │┌────────────┐┏━━━━━━━┓│
                                                      ││ Hola       │┃Publish┃│
                                                      │└────────────┘┗━━━━━━━┛│
                                                      └───────────────────────┘
```

tootik is lightweight, private and accessible social network:
* Its UI is served over [Gemini](https://geminiprotocol.net/), Gopher, Finger and [Guppy](https://github.com/dimkr/guppy-protocol): there's a wide variety of clients to choose from and some work great on old devices.
* Rich content is reduced to plain text and links: it's a fast, low-bandwidth UI suitable for screen readers.
* No promoted content, tracking or analytics: social networking, with the slow and non-commercial vibe of the small internet.
* It's a single static executable, making it easy to [set up your own instance](https://github.com/dimkr/tootik/wiki/Quick-setup-guide) instead of joining an existing one.
* All instance data is stored in a single file, a [sqlite](https://sqlite.org/) database that is easy to backup and restore.
* It's lightweight: a <=$5/mo VPS or a SBC is more than enough for a small instance.
* It implements the subset of ActivityPub required for its feature set but not more, to stay small, reliable and maintainable.
* It's written in two languages ([Go](https://go.dev/) and SQL), making the codebase suitable for educational purposes and easy to hack on.
* It's permissively-licensed.

## Features

* Good compatibility with various fediverse servers
* Text posts, with 3 privacy levels
  * Public
  * To followers
  * To mentioned users
* Sharing of public posts
* Users can follow each other to see non-public posts
  * With support for [Mastodon's follower synchronization mechanism](https://docs.joinmastodon.org/spec/activitypub/#follower-synchronization-mechanism), aka [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md)
* Multi-choice polls
* Full-text search within posts
* Account migration, in both directions

## Using tootik

You can join an [existing instance](gemini://hd.206267.xyz) or [set up your own](https://github.com/dimkr/tootik/wiki/Quick-setup-guide).

## Building

	go generate ./migrations

Then:

	go build ./cmd/tootik -tags fts5

or, to build a static executable:

	go build -tags netgo,sqlite_omit_load_extension,fts5 -ldflags "-linkmode external -extldflags -static" ./cmd/tootik

## Gemini Frontend

* /local shows a compact list of local posts; each entry contains a link to /view.
* / is the homepage: it shows an ASCII art logo, a short description of this server and a list of local posts.
* /federated shows a compact list of federated posts.
* /hashtag shows a compact list of posts with a given hashtag.
* /search shows an input prompt and redirects to /hashtag.
* /hashtags shows a list of popular hashtags.
* /fts shows an input prompt and performs full-text search in posts.
* /stats shows statistics and server health metrics.

* /view shows a complete post with extra details like links in the post, a list mentioned users, a list of hashtags, a link to the author's outbox, a list of replies and a link to the parent post (if found).
* /thread displays a tree of replies in a thread.
* /outbox shows list of posts by a user.

Users are authenticated using TLS client certificates; see [Gemini protocol specification](https://gemini.circumlunar.space/docs/specification.html) for more details. The following pages require authentication:

* /users shows posts by followed users, sorted chronologically.
* /users/mentions is like /users but shows only posts that mention the user.
* /users/register creates a new user.
* /users/follows shows a list of followed users, ordered by activity.
* /users/me redirects the user to their outbox.
* /users/resolve looks up federated user *user@domain* or local user *user*.
* /users/dm creates a post visible to mentioned users.
* /users/whisper creates a post visible to followers.
* /users/say creates a public post.
* /users/reply replies to a post.
* /users/edit edits a post.
* /users/delete deletes a post.
* /users/share shares a post.
* /users/unshare removes a shared post.
* /users/follow sends a follow request to a user.
* /users/unfollow deletes a follow request.
* /users/outbox is equivalent to /outbox but also includes a link to /users/follow or /users/unfollow.
* /users/bio allows users to edit their bio.
* /users/name allows users to set their display name.
* /users/alias allows users to set an account alias, to allow migration of accounts to tootik.
* /users/move allows users to notify followers of account migration from tootik.

Some clients generate a certificate for / (all pages of this capsule) when /foo requests a client certificate, while others use the certificate requested by /foo only for /foo and /foo/bar. Therefore, pages that don't require authentication are also mirrored under /users:

* /users/local
* /users/federated
* /users/hashtag
* /users/hashtags
* /users/fts
* /users/stats
* /users/view
* /users/thread

This way, users who prefer not to provide a client certificate when browsing to /x can reply to public posts by using /users/x instead.

To make the transition to authenticated pages more seamless, links in the user menu at the bottom of each page point to /users/x rather than /x, if the user is authenticated.

All pages follow the [subscription convention](https://gemini.circumlunar.space/docs/companion/subscription.gmi), so users can "subscribe" to a user, a hashtag, posts by followed users or other activity. This way, tootik can act as a personal, curated and prioritized fediverse aggregator. In addition, feeds like /users have separators between days, to interrupt the endless stream of incoming content, make the content consumption more intentional and prevent doomscrolling.

## Authentication

If no client certificate is provided, all pages under /users redirect the client to /users.

/users asks the client to provide a certificate. Well-behaved clients should generate a certificate, re-request /users, then reuse this certificate in future requests of /users and pages under it.

If a certificate is provided but does not belong to any user, the client is redirected to /users/register.

By default, the username associated with a client certificate is the common name specified in the certificate. If invalid or already in use by another user, /users/register asks the user to provide a different username. Once the user is registered, the client is redirected back to /users.

Once the client certificate is associated with a user, all pages under /users look up the authenticated user's data using the certificate hash.

## Posts

tootik has three kinds of posts:
* Messages: posts visible to their author and a single recipient
* Posts: posts visible to followers of a user
* Public posts: posts visible to anyone

User A is allowed to send a message to user B only if B follows A.

### Post Visibility

| Post type          | To                 | CC                              |
|--------------------|--------------------|---------------------------------|
| To mentioned users | -                  | Mentions                        |
| To followers       | Author's followers | Mentions                        |
| To public          | Public             | Mentions and author's followers |

### Reply Visibility

| Post type       | To          | CC                                   |
|-----------------|-------------|--------------------------------------|
| To public       | Post author | Followers of reply author and Public |
| Everything else | Post author | Post audience                        |

### Post Editing

/users/edit cannot remove recipients from the post audience, only add more. If a post that mentions only `@a` is edited to mention only `@b`, both `a` and `b` will receive the updated post.

### Polls

tootik supports [Station](gemini://station.martinrue.com)-style polls. To publish a poll, publish a post in the form:

	[POLL post content] option 1 | option 2 | ...

For example:

	[POLL Does #tootik support polls now?] Yes | No | I don't know

Polls are multi-choice, allowed to have 2 to 5 options and end after a month.

Poll results are updated every 30m and distributed to other servers if needed.

## Implementation Details

### The "Nobody" User

Outgoing requests, like the [WebFinger](https://www.rfc-editor.org/rfc/rfc7033) requests used to discover federated users, are usually associated with a user. For example, the key pair associated with local user A is used to digitally sign the Follow request sent to federated user B.

To protect user's privacy, requests not initiated by a particular user or requests not triggered during handling of user requests (like requests made during validation of incoming public posts) are associated with a special user named "nobody".

### The Resolver

The resolver is responsible for resolving a user ID (local or federated) into an Actor object that contains the user's information, like the user's display name. Actor objects for federated users are cached in the database and updated once in a while.

This is an expensive but common operation that involves outgoing HTTPS requests. Therefore, to protect underpowered servers against heavy load and a big number of concurrent outgoing requests, the maximum number of outgoing requests is capped, concurrent attempts to resolve the same user are blocked and the resolver is a long-lived object that reuses connections.

## Moved Accounts

If a user follows a federated user with the `movedTo` attribute set and the new account's `alsoKnownAs` attribute points back to the old account, follow requests are sent to the new user and old requests are cancelled.

## Notes

The "notes" table holds posts and allows fast search of posts by author, replies to a post and so on.

### Outbox

Once saved to the "notes" table, new posts can be viewed by local users. However, delivery to federated followers can take time and generate many outgoing requests.

Therefore, user actions are represented as an activity saved to the "outbox" table, accompanied by a delivery attempts counter, creation time and last attempt time. For example, a local post saved to the "notes" table is also accompanied with a Create activity in the "outbox" table. A single worker thread polls the table, prioritizes activities by the number of delivery attempts and the interval between attempts, then tries to deliver each activity to its federated recipients.

### Inbox

The server verifies HTTP signatures of requests to /inbox/%s, using the sender's key. They key is cached to reduce the amount of outgoing requests.

The server must resolve unknown users to display their preferred username, summary text and so on, and a user may send an activity that cannot be displayed unless other users associated with it are resolved, too. Therefore, processing of incoming requests can generate outgoing requests. For example, the server must resolve C when A, who follows B and receives replies to B's posts, receives a reply by C, or a post by B which mentions C. B is guaranteed to be cached because A follow B, but C isn't. Therefore, incoming activities are saved to the "inbox" table and a worker thread processes these queued activities.

## Migrations

To add a migration named `x` and add it to the list of migrations:

	./migrations/add.sh x
	go generate ./migrations

## Credits and Legal Information

tootik is free and unencumbered software released under the terms of the [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0); see LICENSE for the license text.

The ASCII art logo at the top was made using [FIGlet](http://www.figlet.org/).
