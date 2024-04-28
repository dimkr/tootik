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
🏕️ Communities
🔥 Hashtags
🔭 Find user
🔎 Search posts
📣 New post
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
* Anonymity: you authenticate using a TLS client certificate and don't have to share your email address or real name.
* No promoted content, tracking or analytics: social networking, with the slow and non-commercial vibe of the small internet.
* It's a single static executable, making it easy to [set up your own instance](https://github.com/dimkr/tootik/wiki/Quick-setup-guide) instead of joining an existing one.
* All instance data is stored in a single file, a [sqlite](https://sqlite.org/) database that is easy to backup and restore.
* It's lightweight: a <=$5/mo VPS or a SBC is more than enough for a small instance.
* It implements the subset of ActivityPub required for its feature set but not more, to stay small, reliable and maintainable.
* It's written in two languages ([Go](https://go.dev/) and SQL), making the codebase suitable for educational purposes and easy to hack on.
* It's permissively-licensed.

## Features

* [Good compatibility with various fediverse servers](FEDERATION.md)
* Text posts, with 3 privacy levels
  * Public
  * To followers
  * To mentioned users
* Sharing of public posts
* Users can follow each other to see non-public posts
  * With support for [Mastodon's follower synchronization mechanism](https://docs.joinmastodon.org/spec/activitypub/#follower-synchronization-mechanism), aka [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md)
* Multi-choice polls
* [Lemmy](https://join-lemmy.org/)-style communities
  * Follow to join
  * Mention community in a public post to start thread
  * Community sends posts and replies to all members
* Full-text search within posts
* Upload of posts and user avatars, over [Titan](gemini://transjovian.org/titan)
* Account migration, in both directions

## Using tootik

You can join an [existing instance](gemini://hd.206267.xyz) or [set up your own](https://github.com/dimkr/tootik/wiki/Quick-setup-guide).

## Building

	go generate ./migrations

Then:

	go build ./cmd/tootik -tags fts5

or, to build a static executable:

	go build -tags netgo,sqlite_omit_load_extension,fts5 -ldflags "-linkmode external -extldflags -static" ./cmd/tootik

## Architecture

```
┌───────┐ ┌────────┐ ┌─────────┐ ┌─────────┐
│ notes │ │ shares │ │ persons │ │ follows │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤
│object │ │note    │ │actor    │ │follower │
│author │ │by      │ │...      │ │followed │
│...    │ │...     │ │         │ │...      │
└───────┘ └────────┘ └─────────┘ └─────────┘
```

Most user-visible data is stored in 4 tables in tootik's database:
1. `notes`, which contains [Object](https://pkg.go.dev/github.com/dimkr/tootik/ap#Object) objects that represent posts
2. `shares`, which records "user A shared post B" relationships
3. `persons`, which contains [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor) objects that represent users
4. `follows`, which records "user A follows user B" relationships

`notes.author`, `shares.by`, `follows.follower` and `follows.followed` point to a row in `persons`.

`shares.note` points to a row in `notes`.

```
┌───────┐ ┌────────┐ ┌─────────┐ ┌─────────┐ ┏━━━━━━━━━┓ ┏━━━━━━━━━┓
│ notes │ │ shares │ │ persons │ │ follows │ ┃ outbox  ┃ ┃  inbox  ┃
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ┣━━━━━━━━━┫ ┣━━━━━━━━━┫
│object │ │note    │ │actor    │ │follower │ ┃activity ┃ ┃activity ┃
│author │ │by      │ │...      │ │followed │ ┃sender   ┃ ┃sender   ┃
│...    │ │...     │ │         │ │...      │ ┃...      ┃ ┃...      ┃
└───────┘ └────────┘ └─────────┘ └─────────┘ ┗━━━━━━━━━┛ ┗━━━━━━━━━┛
```

Federation happens through two tables, `inbox` and `outbox`. Both contain [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) objects that represent actions performed by the users in `persons`.

`inbox` contains activities by users on other servers, while `outbox` contains activities of local users.

```
                    ┏━━━━━━━━━━━━━━━━━┓
                    ┃ gemini.Listener ┃
                    ┗━━━━━━━━┳━━━━━━━━┛
                    ┏━━━━━━━━┻━━━━━━━━┓
                    ┃  front.Handler  ┃
                    ┗━━━━━┳━━━━━━━━━━━┛
┌───────┐ ┌────────┐ ┌────┸────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │
└───────┘ └────────┘ └────┰────┘ └─────────┘ └─────────┘ └─────────┘
                  ┏━━━━━━━┻━━━━━━┓
                  ┃ fed.Resolver ┃
                  ┗━━━━━━━━━━━━━━┛
```

[gemini.Listener](https://pkg.go.dev/github.com/dimkr/tootik/front/gemini#Listener) is a Gemini server that handles requests through [Handler](https://pkg.go.dev/github.com/dimkr/tootik/front#Handler). It adds rows to `persons` during new user registration and changes rows when users change properties like their display name.

[Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) is responsible for fetching [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor)s that represents users of other servers. The fetched objects are cached in `persons`.

```
                ┌─────────────────┐
                │ gemini.Listener │
                └────────┬────────┘
                ┌────────┴─────────┐
    ┏━━━━━━━━━━━┥  front.Handler   │
    ┃           └┰────────┬───────┰┘
┌───┸───┐ ┌──────┸─┐ ┌────┴────┐ ┌┸────────┐ ┌─────────┐ ┌─────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │
└───────┘ └────────┘ └────┬────┘ └─────────┘ └─────────┘ └─────────┘
                  ┌───────┴──────┐
                  │ fed.Resolver │
                  └──────────────┘
```

In addition, Gemini requests can:
* Add rows to `notes` (new post)
* Change rows in `notes` (post editing)
* Add rows to `shares` (user shares a post)
* Remove rows from `shares` (user no longer shares a post)
* Add rows to `follows` (user A followed user B)
* Remove rows from `follows` (user A unfollowed user B)
* ...

```
                ┌─────────────────┐
                │ gemini.Listener │
                └────────┬────────┘
                ┌────────┴─────────┐
    ┌───────────┤  front.Handler   ┝━━━━━━━━━━━┓
    │           └┬────────┬───────┬┘           ┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴────────┐ ┌─┸───────┐ ┌─────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │
└───────┘ └────────┘ └────┬────┘ └───────┰─┘ └┰────────┘ └─────────┘
                  ┌───────┴──────┐      ┏┻━━━━┻━━━━━┓
                  │ fed.Resolver │      ┃ fed.Queue ┃
                  └──────────────┘      ┗━━━━━━━━━━━┛
```

Each user action (post creation, post deletion, ...) is recorded as an [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) object written to `outbox`.

[fed.Queue](https://pkg.go.dev/github.com/dimkr/tootik/fed#Queue) is responsible for sending activities to followers from other servers, if needed.

```
                                      ┏━━━━━━━━━━━━━━━┓
                ┌─────────────────┐   ┃ outbox.Mover  ┃
                │ gemini.Listener │   ┃ outbox.Poller ┃
                └────────┬────────┘   ┃ fed.Syncer    ┃
                ┌────────┴─────────┐  ┗━━━┳━━━━━┳━━━━━┛
    ┌───────────┤  front.Handler   ├──────╂────┐┃
    │           └┬────────┬───────┬┘      ┃    │┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┸┐ ┌─┴┸──────┐ ┌─────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │
└───────┘ └────────┘ └────┬────┘ └───────┬─┘ └┬────────┘ └─────────┘
                  ┌───────┴──────┐      ┌┴────┴─────┐
                  │ fed.Resolver │      │ fed.Queue │
                  └──────────────┘      └───────────┘
```

tootik may perform automatic actions in the name of the user:
1. Follow the new account and unfollow the old one, if a followed user moved their account
2. Update poll results for polls published by the user, and send the new results to followers
3. Handle disagreement between `follows` rows for this user and what other servers know

```
                                      ┌───────────────┐
                ┌─────────────────┐   │ outbox.Mover  │
                │ gemini.Listener │   │ outbox.Poller │
                └────────┬────────┘   │ fed.Syncer    │
                ┌────────┴─────────┐  └───┬─────┬─────┘ ┏━━━━━━━━━━━━━━┓
    ┌───────────┤  front.Handler   ├──────┼────┐│    ┏━━┫ fed.Listener ┣━━┓
    │           └┬────────┬───────┬┘      │    ││    ┃  ┗━━━━━┳━━━━━━━━┛  ┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┸─┐ ┌────┸────┐      ┃
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │      ┃
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤      ┃
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │      ┃
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │      ┃
│...    │ │...     │ │         │ │...      │ │...      │ │...      │      ┃
└───────┘ └────────┘ └────┬────┘ └───────┬─┘ └┬────────┘ └─────────┘      ┃
                  ┌───────┴──────┐      ┌┴────┴─────┐                     ┃
                  │ fed.Resolver │      │ fed.Queue │                     ┃
                  └───────┰──────┘      └───────────┘                     ┃
                          ┃                                               ┃
                          ┃                                               ┃
                          ┃                                               ┃
                          ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

Requests from other servers are handled by [fed.Listener](https://pkg.go.dev/github.com/dimkr/tootik/fed#Listener), a HTTP server.

It extracts the signature and key ID from a request using [httpsig.Extract](https://pkg.go.dev/github.com/dimkr/tootik/httpsig#Extract), uses [Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) to fetch the public key if needed, validates the request using [Verify](https://pkg.go.dev/github.com/dimkr/tootik/httpsig#Signature.Verify) and inserts the received [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) object into `inbox`.

In addition, [fed.Listener](https://pkg.go.dev/github.com/dimkr/tootik/fed#Listener) allows other servers to fetch public activity (like public posts) from `outbox`, so they can fetch some past activity by a newly-followed user.

```
                                      ┌───────────────┐
                ┌─────────────────┐   │ outbox.Mover  │
                │ gemini.Listener │   │ outbox.Poller │
                └────────┬────────┘   │ fed.Syncer    │
                ┌────────┴─────────┐  └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤  front.Handler   ├──────┼────┐│    ┌──┤ fed.Listener ├──┐
    │           └┬────────┬───────┬┘      │    ││    │  └─────┬────────┘  │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴─┐ ┌────┴────┐      │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │      │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤      │
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │      │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │      │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │      │
└───┰───┘ └───┰────┘ └────┬────┘ └────┰──┬─┘ └┬────────┘ └──────┰──┘      │
    ┃         ┃   ┌───────┴──────┐    ┃ ┌┴────┴─────┐     ┏━━━━━┻━━━━━━━┓ │
    ┃         ┃   │ fed.Resolver │    ┃ │ fed.Queue │     ┃ inbox.Queue ┃ │
    ┃         ┃   └───────┬──────┘    ┃ └───────────┘     ┗━┳━┳━┳━━━━━━━┛ │
    ┃         ┃           │           ┗━━━━━━━━━━━━━━━━━━━━━┛ ┃ ┃         │
    ┃         ┗━━━━━━━━━━━┿━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ ┃         │
    ┗━━━━━━━━━━━━━━━━━━━━━┿━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛         │
                          └───────────────────────────────────────────────┘
```

Once inserted into `inbox`, [inbox.Queue](https://pkg.go.dev/github.com/dimkr/tootik/inbox#Queue) processes the received activities:
* Adds new posts received in `Create` activities to `notes`
* Edits post in `notes` according to `Update` activities
* Records `Announce` activities in `shares`
* Marks a follower-followed relationship in `follows` as accepted, when the followed user sends an `Accept` activity
* Adds a new row to `follows` when a remote user sends a `Follow` activity to a local user
* ...

```
                                      ┌───────────────┐
                ┌─────────────────┐   │ outbox.Mover  │
                │ gemini.Listener │   │ outbox.Poller │
                └────────┬────────┘   │ fed.Syncer    │
                ┌────────┴─────────┐  └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤  front.Handler   ├──────┼────┐│    ┌──┤ fed.Listener ├──┐
    │           └┬────────┬───────┬┘      │    ││    │  └─────┬────────┘  │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴─┐ ┌────┴────┐      │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │      │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤      │
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │      │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │      │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │      │
└───┬───┘ └───┬────┘ └────┬────┘ └────┬──┬─┘ └┬───────┰┘ └──────┬──┘      │
    │         │   ┌───────┴──────┐    │ ┌┴────┴─────┐ ┃   ┌─────┴───────┐ │
    │         │   │ fed.Resolver │    │ │ fed.Queue │ ┗━━━┥ inbox.Queue │ │
    │         │   └───────┬──────┘    │ └───────────┘     └─┬─┬─┬───────┘ │
    │         │           │           └─────────────────────┘ │ │         │
    │         └───────────┼───────────────────────────────────┘ │         │
    └─────────────────────┼─────────────────────────────────────┘         │
                          └───────────────────────────────────────────────┘
```

Sometimes, a received or newly created local [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is forwarded to the followers of a local user:
* When a remote user replies in a thread started by a local user, the received [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is inserted into `outbox` and forwarded to all followers of the local user.
* When a user creates a new post, edits a post or deletes a post in a local community, the [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is wrapped with an `Announce` activity that's inserted into `outbox` and forwarded to all community members.

```
                                      ┌───────────────┐
                ┌─────────────────┐   │ outbox.Mover  │
                │ gemini.Listener │   │ outbox.Poller │
                └────────┬────────┘   │ fed.Syncer    │
                ┌────────┴─────────┐  └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤  front.Handler   ├──────┼────┐│    ┌──┤ fed.Listener ├──┐
    │           └┬────────┬───────┬┘      │    ││    │  └─────┬────────┘  │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴─┐ ┌────┴────┐      │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox  │ │  inbox  │      │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├─────────┤ ├─────────┤      │
│object │ │note    │ │actor    │ │follower │ │activity │ │activity │      │
│author │ │by      │ │...      │ │followed │ │sender   │ │sender   │      │
│...    │ │...     │ │         │ │...      │ │...      │ │...      │      │
└───┬───┘ └───┬────┘ └────┬────┘ └────┬──┬─┘ └┬───────┬┘ └──────┬──┘      │
    │         │   ┌───────┴──────┐    │ ┌┴────┴─────┐ │   ┌─────┴───────┐ │
    │         │   │ fed.Resolver │    │ │ fed.Queue │ └───┤ inbox.Queue │ │
    │         │   └───────┬─┰────┘    │ └───────────┘     └─┬─┬─┬─┰─────┘ │
    │         │           │ ┃         └─────────────────────┘ │ │ ┃       │
    │         └───────────┼─╂─────────────────────────────────┘ │ ┃       │
    └─────────────────────┼─╂───────────────────────────────────┘ ┃       │
                          └─╂─────────────────────────────────────╂───────┘
                            ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

To display details like the user's name and speed up the verification of future incoming replies, [inbox.Queue](https://pkg.go.dev/github.com/dimkr/tootik/inbox#Queue) uses [Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) to fetch the [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor) objects of mentioned users (if needed).

## More Documentation

* [Frontend](front/README.md)
* [Migrations](migrations/README.md)
* [Compatibility](FEDERATION.md)

## Credits and Legal Information

tootik is free and unencumbered software released under the terms of the [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0); see LICENSE for the license text.

The ASCII art logo at the top was made using [FIGlet](http://www.figlet.org/).
