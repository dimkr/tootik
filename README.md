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

Welcome, fedinaut! localhost.localdomain:8443 is a text-based social network.

────

📻 My feed
📞 Mentions
⚡️ Follows
🐕 Followers
😈 My profile
📡 Local feed
🏕️ Communities
🔥 Hashtags
🔭 View profile
🔖 Bookmarks
🔎 Search posts
📣 New post
⚙️ Settings
📊 Status
🛟 Help
```

[![Latest release](https://img.shields.io/github/v/release/dimkr/tootik)](https://github.com/dimkr/tootik/releases) [![Build status](https://github.com/dimkr/tootik/actions/workflows/ci.yml/badge.svg)](https://github.com/dimkr/tootik/actions) [![Go Reference](https://pkg.go.dev/badge/github.com/dimkr/tootik.svg)](https://pkg.go.dev/github.com/dimkr/tootik)

## Overview

tootik is a text-based social network.

tootik is federated using [ActivityPub](https://www.w3.org/TR/activitypub/): users can join an existing instance or [set up](SETUP.md) their own, then interact with users on the same instance and users of [compatible servers](FEDERATION.md) like [Mastodon](https://joinmastodon.org/), [Lemmy](https://join-lemmy.org/), [Sharkey](https://activitypub.software/TransFem-org/Sharkey), [Friendica](https://friendi.ca/), [Akkoma](https://akkoma.dev/AkkomaGang/akkoma/), [GoToSocial](https://gotosocial.org/) and [Mitra](https://codeberg.org/silverpill/mitra).

Unlike other social networks, tootik doesn't have a browser-based interface or an app: instead, its minimalistic, text-based interface is served over [Gemini](https://geminiprotocol.net/):

```
                         Gemini           ActivityPub (HTTPS)
                           →                     ⇄
 ┏━━━━━━━━━━━━━━━━━━━━━━━┓   ┏━━━━━━━━━━━━━━━━━┓   ┌─────────────────────────┐
 ┃  Bob's Gemini client  ┣━━━┫ tootik instance ┠─┬─┤ Another tootik instance │
 ┣━━━━━━━━━━━━━━━━━━━━━━━┫   ┣━━━━━━━━━━━━━━━━━┫ │ └─────────────────────────┘
 ┃2024-01-01 alice       ┃   ┃$ ./tootik ...   ┃ │ ┌────────────────┐
 ┃> Hi @bob and @carol!  ┃   ┗━┳━━━━━━━━━━━━━━━┛ ├─┤ Something else │
 ┃...                    ┃     ┃                 │ └────────────────┘
 ┗━━━━━━━━━━━━━━━━━━━━━━━┛     ┃                 │ ┌───────────────────┐
               ┏━━━━━━━━━━━━━━━┻━━━━━━━┓         └─┤ Mastodon instance ├─┐
               ┃ Alice's Gemini client ┃           └───────────────────┘ │
               ┣━━━━━━━━━━━━━━━━━━━━━━━┫              ┌──────────────────┴────┐
               ┃2024-01-01 bob         ┃              │  Carol's web browser  │
               ┃> Hi @alice!           ┃              ├───────────────────────┤
               ┃...                    ┃              │╔═╗ alice              │
               ┗━━━━━━━━━━━━━━━━━━━━━━━┛              │╚═╝             17h ago│
                                                      │Hi @bob and @carol!    │
                                                      │                       │
                                                      │  ╔═╗ bob              │
                                                      │  ╚═╝           16h ago│
                                                      │  Hi @alice!           │
                                                      ├───────────────────────┤
                                                      │┌────────────┐┌───────┐│
                                                      ││ Hola       ││Publish││
                                                      │└────────────┘└───────┘│
                                                      └───────────────────────┘
```

This makes tootik lightweight, private and accessible:
* Its UI supports [Gemini](https://geminiprotocol.net/) but also Gopher, Finger and [Guppy](https://github.com/dimkr/guppy-protocol): there's a wide variety of clients to choose from and some work great on old devices.
* Rich content is reduced to plain text and links: it's a fast, low-bandwidth UI suitable for screen readers.
* Anonymity: you authenticate using a TLS client certificate and don't have to share your email address or real name.
* No promoted content, tracking or analytics: social networking, with the slow and non-commercial vibe of the small internet.
* It's a single static executable, making it easy to [set up your own instance](SETUP.md) and update it later.
* All instance data is stored in a single file, a [sqlite](https://sqlite.org/) database that is easy to backup and restore.
* It's lightweight: a <=$5/mo VPS or a SBC is more than enough for a small instance.
* It implements the subset of ActivityPub required for its feature set but not more, to stay small, reliable and maintainable.
* It's written from scratch (not forked from some other project) in two languages ([Go](https://go.dev/) and SQL), making the codebase suitable for educational purposes and easy to hack on.
* It's permissively-licensed.

## Features

* [Good compatibility with various fediverse servers](FEDERATION.md)
* Text posts, with 3 privacy levels
  * Public
  * To followers
  * To mentioned users
* Sharing of public posts
* [FEP-044f](https://codeberg.org/fediverse/fep/src/branch/main/fep/044f/fep-044f.md) quote posts, without support for approval
* Users can follow each other to see non-public posts
  * With support for manual approval of follow requests
  * With support for [Mastodon's follower synchronization mechanism](https://docs.joinmastodon.org/spec/activitypub/#follower-synchronization-mechanism), aka [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md)
* [FEP-ef61](https://codeberg.org/fediverse/fep/src/branch/main/fep/ef61/fep-ef61.md) portable accounts
* Multi-choice polls
* [Lemmy](https://join-lemmy.org/)-style communities
  * Follow to join
  * Mention community in a public post to start thread
  * Community sends posts and replies to all members
* Bookmarks
* Full-text search within posts
* Upload of posts and user avatars, over [Titan](gemini://transjovian.org/titan)
* Automatic deletion of old posts
* Account migration, in both directions
* Support for multiple client certificates

## Using tootik

You can join an [existing instance](gemini://hd.206267.xyz) or [set up your own](SETUP.md).

## Building

	go generate ./migrations

Then:

	go build ./cmd/tootik -tags fts5

or, to build a static executable:

	go build -tags netgo,sqlite_omit_load_extension,fts5 -ldflags "-linkmode external -extldflags -static" ./cmd/tootik

## Architecture

```
┏━━━━━━━┓ ┏━━━━━━━━┓ ┏━━━━━━━━━┓ ┏━━━━━━━━━┓
┃ notes ┃ ┃ shares ┃ ┃ persons ┃ ┃ follows ┃
┣━━━━━━━┫ ┣━━━━━━━━┫ ┣━━━━━━━━━┫ ┣━━━━━━━━━┫
┃object ┃ ┃note    ┃ ┃actor    ┃ ┃follower ┃
┃author ┃ ┃by      ┃ ┃...      ┃ ┃followed ┃
┃...    ┃ ┃...     ┃ ┃         ┃ ┃...      ┃
┗━━━━━━━┛ ┗━━━━━━━━┛ ┗━━━━━━━━━┛ ┗━━━━━━━━━┛
```

Most user-visible data is stored in 4 tables in tootik's database:
1. `notes`, which contains [Object](https://pkg.go.dev/github.com/dimkr/tootik/ap#Object) objects that represent posts
2. `shares`, which records "user A shared post B" relationships
3. `persons`, which contains [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor) objects that represent users
4. `follows`, which records "user A follows user B" relationships

`notes.author`, `shares.by`, `follows.follower` and `follows.followed` point to rows in `persons`.

`shares.note` points to a row in `notes`.

```
┌───────┐ ┌────────┐ ┌─────────┐ ┌─────────┐ ┏━━━━━━━━┓ ┏━━━━━━━━┓
│ notes │ │ shares │ │ persons │ │ follows │ ┃ outbox ┃ ┃ inbox  ┃
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ┣━━━━━━━━┫ ┣━━━━━━━━┫
│object │ │note    │ │actor    │ │follower │ ┃activity┃ ┃activity┃
│author │ │by      │ │...      │ │followed │ ┃sender  ┃ ┃sender  ┃
│...    │ │...     │ │         │ │...      │ ┃...     ┃ ┃...     ┃
└───────┘ └────────┘ └─────────┘ └─────────┘ ┗━━━━━━━━┛ ┗━━━━━━━━┛
```

Federation happens through two tables, `inbox` and `outbox`. Both contain [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) objects that represent actions performed by the users in `persons`.

`inbox` contains activities by users on other servers, while `outbox` contains activities of local users.

```
  ┏━━━━━━━━━━┓ ┏━━━━━━━━━━━━━━━━━┓
  ┃ gmi.Wrap ┣━┫ gemini.Listener ┃
  ┗━━━━━━━━━━┛ ┗━━━━━━━━┳━━━━━━━━┛
                ┏━━━━━━━┻━━━━━━━━━━━┓
                ┃   front.Handler   ┃
                ┗━━━━━━━━━┳━━━━━━━━━┛
┌───────┐ ┌────────┐ ┌────┸────┐ ┌─────────┐ ┌────────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └─────────┘ └─────────┘ └────────┘ └────────┘
```

[gemini.Listener](https://pkg.go.dev/github.com/dimkr/tootik/front/gemini#Listener) is a Gemini server that handles requests using [Handler](https://pkg.go.dev/github.com/dimkr/tootik/front#Handler). It adds rows to `persons` during new user registration and changes rows when users change properties like their display name.

[gemini.Listener](https://pkg.go.dev/github.com/dimkr/tootik/front/gemini#Listener) provides [Handler](https://pkg.go.dev/github.com/dimkr/tootik/front#Handler) with a [writer](https://pkg.go.dev/github.com/dimkr/tootik/front/text/gmi#Wrap) that builds a Gemini response and asynchronously sends it to the client in chunks, while [Handler](https://pkg.go.dev/github.com/dimkr/tootik/front#Handler) continues to handle the request and append more lines to the page.

```
  ┌──────────┐ ┌─────────────────┐
  │ gmi.Wrap ├─┤ gemini.Listener │
  └──────────┘ └────────┬────────┘
                ┌───────┴───────────┐
    ┏━━━━━━━━━━━┥   front.Handler   │
    ┃           └┰────────┬───────┰─┘
┌───┸───┐ ┌──────┸─┐ ┌────┴────┐ ┌┸────────┐ ┌────────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └─────────┘ └─────────┘ └────────┘ └────────┘
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
  ┌──────────┐ ┌─────────────────┐
  │ gmi.Wrap ├─┤ gemini.Listener │
  └──────────┘ └────────┬────────┘
                ┌───────┴───────────┐
    ┌───────────┤   front.Handler   ┝━━━━━━━━━━┓
    │           └┬────────┬───────┬─┘          ┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴────────┐ ┌─┸──────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └─────────┘ └─────────┘ └────────┘ └────────┘
```

User actions like post creation or deletion are recorded as [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) objects written to `outbox`.

```
                                      ┏━━━━━━━━━━━━━━━┓
  ┌──────────┐ ┌─────────────────┐    ┃ outbox.Mover  ┃
  │ gmi.Wrap ├─┤ gemini.Listener │    ┃ outbox.Poller ┃
  └──────────┘ └────────┬────────┘    ┃ fed.Syncer    ┃
                ┌───────┴───────────┐ ┗━━━┳━━━━━┳━━━━━┛
    ┌───────────┤   front.Handler   ├─────╂────┐┃
    │           └┬────────┬───────┬─┘     ┃    │┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┸┐ ┌─┴┸─────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └─────────┘ └─────────┘ └────────┘ └────────┘
```

tootik may perform automatic actions and push additional activities to `outbox`, on behalf of the user:
1. Follow the new account and unfollow the old one, if a followed user moved their account
2. Update poll results for polls published by the user, and send the new results to followers
3. Handle disagreement between `follows` rows for this user and what other servers know

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘
    ┌───────────┤   front.Handler   ├─────┼────┐│
    │           └┬────────┬───────┬─┘     │    ││
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴─────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └─────────┘ └───────┰─┘ └┰───────┘ └────────┘
                                        ┏┻━━━━┻━━━━━┓
                                        ┃ fed.Queue ┃
                                        ┗━━━━━━━━━━━┛
```

[fed.Queue](https://pkg.go.dev/github.com/dimkr/tootik/fed#Queue) polls `outbox` and delivers these activities to followers on other servers. It uses the `deliveries` table to track delivery progress and retry failed deliveries.

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘
    ┌───────────┤   front.Handler   ├─────┼────┐│
    │           └┬────────┬───────┬─┘     │    ││
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴─────┐ ┌────────┐
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │
└───────┘ └────────┘ └────┰────┘ └───────┬─┘ └┬───────┘ └────────┘
                          ┃             ┌┴────┴─────┐
                  ┏━━━━━━━┻━━━━━━┓ ┏━━━━┥ fed.Queue │
                  ┃ fed.Resolver ┣━┛    └───────────┘
                  ┗━━━━━━━━━━━━━━┛
```

[Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) is responsible for fetching [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor)s that represent users of other servers, using an ID or a `user@domain` pair and [WebFinger](https://datatracker.ietf.org/doc/html/rfc7033). The fetched objects are cached in `persons`, and contain properties like the user's inbox URL and public key.

[fed.Queue](https://pkg.go.dev/github.com/dimkr/tootik/fed#Queue) uses [Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) to make a list of unique inbox URLs each activity should be delivered to. If this is a wide delivery (a public post or a post to followers) and two recipients share the same `sharedInbox`, [fed.Queue](https://pkg.go.dev/github.com/dimkr/tootik/fed#Queue) delivers the activity to both recipients in a single request.

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘ ┏━━━━━━━━━━━━━━┓
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┏━━┫ fed.Listener ┣━━━━━━┓
    │           └┬────────┬───────┬─┘     │    ││    ┃  ┗━━━━━┳━━━━━━━━┛      ┃
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┸┐ ┌─────┸──┐            ┃
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │            ┃
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤            ┃
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│            ┃
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │            ┃
│...    │ │...     │ │         │ │...      │ │...     │ │...     │            ┃
└───────┘ └────────┘ └────┬────┘ └───────┬─┘ └┬───────┘ └────────┘            ┃
                          │             ┌┴────┴─────┐                         ┃
                  ┌───────┴──────┐ ┌────┤ fed.Queue │                         ┃
                  │ fed.Resolver ├─┘    └───────────┘                         ┃
                  └───────┰──────┘                                            ┃
                          ┃                                                   ┃
                          ┃                                                   ┃
                          ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

Requests from other servers are handled by [fed.Listener](https://pkg.go.dev/github.com/dimkr/tootik/fed#Listener), a HTTP server.

It extracts the signature and key ID from a request using [httpsig.Extract](https://pkg.go.dev/github.com/dimkr/tootik/httpsig#Extract), uses [Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) to fetch the public key if needed, validates the request using [Verify](https://pkg.go.dev/github.com/dimkr/tootik/httpsig#Signature.Verify) and inserts the received [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) object into `inbox`.

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┌──┤ fed.Listener ├──────┐
    │           └┬────────┬───────┬─┘     │    ││    │  └─────┬────────┘      │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴┐ ┌─────┴──┐            │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │            │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤            │
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│            │
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │            │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │            │
└───┰───┘ └───┰────┘ └────┬────┘ └────┰──┬─┘ └┬───────┘ └──────┰─┘            │
    ┃         ┃           │           ┃ ┌┴────┴─────┐     ┏━━━━┻━━━━━━━━┓     │
    ┃         ┃   ┌───────┴──────┐ ┌──╂─┤ fed.Queue │     ┃ inbox.Queue ┃     │
    ┃         ┃   │ fed.Resolver ├─┘  ┃ └───────────┘     ┗━┳━┳━┳━━━━━━━┛     │
    ┃         ┃   └───────┬──────┘    ┗━━━━━━━━━━━━━━━━━━━━━┛ ┃ ┃             │
    ┃         ┗━━━━━━━━━━━┿━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ ┃             │
    ┗━━━━━━━━━━━━━━━━━━━━━┿━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛             │
                          └───────────────────────────────────────────────────┘
```

Once inserted into `inbox`, [inbox.Queue](https://pkg.go.dev/github.com/dimkr/tootik/inbox#Queue) processes the received activities:
* Adds new posts received in `Create` activities to `notes`
* Edits posts in `notes` according to `Update` activities
* Records `Announce` activities in `shares`
* Marks a follower-followed relationship in `follows` as accepted, when the followed user sends an `Accept` activity
* Adds a new row to `follows` when a remote user sends a `Follow` activity to a local user
* ...

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┌──┤ fed.Listener ├──────┐
    │           └┬────────┬───────┬─┘     │    ││    │  └─────┬────────┘      │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴┐ ┌─────┴──┐            │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │            │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤            │
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│            │
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │            │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │            │
└───┬───┘ └───┬────┘ └────┬────┘ └────┬──┬─┘ └┬──────┰┘ └──────┬─┘            │
    │         │           │           │ ┌┴────┴─────┐┃    ┌────┴────────┐     │
    │         │   ┌───────┴──────┐ ┌──┼─┤ fed.Queue │┗━━━━┥ inbox.Queue │     │
    │         │   │ fed.Resolver ├─┘  │ └───────────┘     └─┬─┬─┬───────┘     │
    │         │   └───────┬──────┘    └─────────────────────┘ │ │             │
    │         └───────────┼───────────────────────────────────┘ │             │
    └─────────────────────┼─────────────────────────────────────┘             │
                          └───────────────────────────────────────────────────┘
```

Sometimes, a received or newly created local [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is forwarded to the followers of a local user:
* When a remote user replies in a thread started by a local user, the received [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is inserted into `outbox` and forwarded to all followers of the local user.
* When a user creates a new post, edits a post or deletes a post in a local community, the [Activity](https://pkg.go.dev/github.com/dimkr/tootik/ap#Activity) is inserted into `outbox` and forwarded to all community members.

```
                                      ┌───────────────┐
  ┌──────────┐ ┌─────────────────┐    │ outbox.Mover  │
  │ gmi.Wrap ├─┤ gemini.Listener │    │ outbox.Poller │
  └──────────┘ └────────┬────────┘    │ fed.Syncer    │
                ┌───────┴───────────┐ └───┬─────┬─────┘ ┌──────────────┐
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┌──┤ fed.Listener ├──────┐
    │           └┬────────┬───────┬─┘     │    ││    │  └─────┬────────┘      │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴┐ ┌─────┴──┐            │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │            │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤            │
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│            │
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │            │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │            │
└───┬───┘ └───┬────┘ └────┬────┘ └────┬──┬─┘ └┬──────┬┘ └──────┬─┘            │
    │         │           │           │ ┌┴────┴─────┐│    ┌────┴────────┐     │
    │         │   ┌───────┴──────┐ ┌──┼─┤ fed.Queue │└────┤ inbox.Queue │     │
    │         │   │ fed.Resolver ├─┘  │ └───────────┘     └─┬─┬─┬─┰─────┘     │
    │         │   └───────┬─┰────┘    └─────────────────────┘ │ │ ┃           │
    │         └───────────┼─╂─────────────────────────────────┘ │ ┃           │
    └─────────────────────┼─╂───────────────────────────────────┘ ┃           │
                          └─╂─────────────────────────────────────╂───────────┘
                            ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

To display details like the user's name and speed up the verification of future incoming replies, [inbox.Queue](https://pkg.go.dev/github.com/dimkr/tootik/inbox#Queue) uses [Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver) to fetch the [Actor](https://pkg.go.dev/github.com/dimkr/tootik/ap#Actor) objects of mentioned users (if needed).

```
                                   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
                                   ┃  ┌───────────────┐                   ┃
  ┌──────────┐ ┌─────────────────┐ ┃  │ outbox.Mover  │                   ┃
  │ gmi.Wrap ├─┤ gemini.Listener │ ┃  │ outbox.Poller │                   ┃
  └──────────┘ └────────┬────────┘ ┃  │ fed.Syncer    │                   ┃
                ┌───────┴──────────┸┐ └───┬─────┬─────┘ ┌──────────────┐  ┃
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┌──┤ fed.Listener ├──╂───┐
    │           └┬────────┬───────┬─┘     │    ││    │  └─────┬────────┘  ┃   │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴┐ ┌─────┴──┐ ┏━━━━━━┻━┓ │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │ ┃  feed  ┃ │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤ ┣━━━━━━━━┫ │
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│ ┃follower┃ │
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │ ┃note    ┃ │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │ ┃...     ┃ │
└─┰─┬───┘ └─┰─┬────┘ └──┰─┬────┘ └──┰─┬──┬─┘ └┬──────┬┘ └──────┬─┘ ┗━━━━━━┳━┛ │
  ┃ │       ┃ │ ┏━━━━━━━┛ │         ┃ │ ┌┴────┴─────┐│    ┌────┴────────┐ ┃   │
  ┃ │       ┃ │ ┃ ┌───────┴──────┐ ┌╂─┼─┤ fed.Queue │└────┤ inbox.Queue │ ┃   │
  ┃ │       ┃ │ ┃ │ fed.Resolver ├─┘┃ │ └───────────┘     └─┬─┬─┬─┬─────┘ ┃   │
  ┃ │       ┃ │ ┃ └───────┬─┬────┘  ┃ └─────────────────────┘ │ │ │       ┃   │
  ┃ │       ┃ └─╂─────────┼─┼───────╂─────────────────────────┘ │ │       ┃   │
  ┃ └───────╂───╂─────────┼─┼───────╂───────────────────────────┘ │       ┃   │
  ┃         ┃   ┃         └─┼───────╂─────────────────────────────┼───────╂───┘
┏━┻━━━━━━━━━┻━━━┻━━━┓       └───────╂─────────────────────────────┘       ┃
┃ inbox.FeedUpdater ┣━━━━━━━━━━━━━━━┛                                     ┃
┗━━━━━━━━━┳━━━━━━━━━┛                                                     ┃
          ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

To speed up each user's feed, [inbox.FeedUpdater](https://pkg.go.dev/github.com/dimkr/tootik/inbox#FeedUpdater) periodically appends rows to the `feed` table. This table holds all information that appears in the user's feed: posts written or shared by followed users, author information and more, eliminating the need for `join` queries, slow filtering by post visibility, deduplication and sorting by time when a user views their feed. This table is indexed by user and time, allowing fast querying of a single feed page for a particular user.

```
                                      ┏━━━━━━━━━━━━━━━━┓
                        ┏━━━━━━━━━━━━━┫ cluster.Server ┣━━━━━━┓
                        ┃             ┗━━━━━━━━━━━━━━━━┛      ┃
                        ┃          ┌──────────────────────────╂───────────┐
                        ┃          │  ┌───────────────┐       ┃           │
  ┌──────────┐ ┌────────┸────────┐ │  │ outbox.Mover  │       ┃           │
  │ gmi.Wrap ├─┤ gemini.Listener │ │  │ outbox.Poller │       ┃           │
  └──────────┘ └────────┬────────┘ │  │ fed.Syncer    │       ┃           │
                ┌───────┴──────────┴┐ └───┬─────┬─────┘ ┌─────┸────────┐  │
    ┌───────────┤   front.Handler   ├─────┼────┐│    ┌──┤ fed.Listener ├──┼───┐
    │           └┬────────┬───────┬─┘     │    ││    │  └─────┬────────┘  │   │
┌───┴───┐ ┌──────┴─┐ ┌────┴────┐ ┌┴───────┴┐ ┌─┴┴────┴┐ ┌─────┴──┐ ┌──────┴─┐ │
│ notes │ │ shares │ │ persons │ │ follows │ │ outbox │ │ inbox  │ │  feed  │ │
├───────┤ ├────────┤ ├─────────┤ ├─────────┤ ├────────┤ ├────────┤ ├────────┤ │
│object │ │note    │ │actor    │ │follower │ │activity│ │activity│ │follower│ │
│author │ │by      │ │...      │ │followed │ │sender  │ │sender  │ │note    │ │
│...    │ │...     │ │         │ │...      │ │...     │ │...     │ │...     │ │
└─┬─┬───┘ └─┬─┬────┘ └──┬─┬────┘ └──┬─┬──┬─┘ └┬──────┬┘ └──────┬─┘ └──────┬─┘ │
  │ │       │ │ ┌───────┘ │         │ │ ┌┴────┴─────┐│    ┌────┴────────┐ │   │
  │ │       │ │ │ ┌───────┴──────┐ ┌┼─┼─┤ fed.Queue │└────┤ inbox.Queue │ │   │
  │ │       │ │ │ │ fed.Resolver ├─┘│ │ └───────────┘     └─┬─┬─┬─┬─────┘ │   │
  │ │       │ │ │ └─────┰─┬─┬────┘  │ └─────────────────────┘ │ │ │       │   │
  │ │       │ └─┼───────╂─┼─┼───────┼─────────────────────────┘ │ │       │   │
  │ └───────┼───┼───────╂─┼─┼───────┼───────────────────────────┘ │       │   │
  │         │   │       ┃ └─┼───────┼─────────────────────────────┼───────┼───┘
┌─┴─────────┴───┴───┐   ┃   └───────┼─────────────────────────────┘       │
│ inbox.FeedUpdater ├───╂───────────┘                                     │
└─────────┬─────────┘   ┃                                                 │
          └─────────────╂─────────────────────────────────────────────────┘
               ┏━━━━━━━━┻━━━━━━━┓
               ┃ cluster.Client ┃
               ┗━━━━━━━━━━━━━━━━┛
```

The [cluster](https://pkg.go.dev/github.com/dimkr/tootik/cluster) package contains complex tests that simulate interaction between users on multiple servers. These tests are easy to write, they're fast and they run in parallel without affecting each other.

The tests use three main constructs: [Client](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Client), [Server](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Server) and [Cluster](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Cluster).

During tests, all [http.Request](https://pkg.go.dev/net/http#Request)s sent by tootik (like those sent by [fed.Resolver](https://pkg.go.dev/github.com/dimkr/tootik/fed#Resolver)) are sent through [Client](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Client).

[Server](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Server) handles all kinds of incoming requests:
* It uses a Unix socket wrapped with TLS and [gemini.Listener](https://pkg.go.dev/github.com/dimkr/tootik/front/gemini#Listener) to allow tests to simulate interaction with the [Gemini](https://geminiprotocol.net/) interface, including user authentication through client certificates
* It uses the same [http.Handler](https://pkg.go.dev/net/http#Handler) as [fed.Listener](https://pkg.go.dev/github.com/dimkr/tootik/front/fed#Listener) to handle an incoming [http.Request](https://pkg.go.dev/net/http#Request) but without needing an actual HTTP server

[Client](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Client) holds a mapping between domain names and [Server](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Server)s: it allows these servers to talk to each other by passing the [http.Request](https://pkg.go.dev/net/http#Request) sent by one server to the [http.Handler](https://pkg.go.dev/net/http#Handler) of another.

[Cluster](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Cluster) is a high-level wrapper for easy creation of multiple [Server](https://pkg.go.dev/github.com/dimkr/tootik/cluster#Server)s capable of federating with each other, given a list of domain names.

## More Documentation

* [Setup guide](SETUP.md)
* [Frontend](front/README.md)
* [Migrations](migrations/README.md)
* [Compatibility](FEDERATION.md)

## Credits and Legal Information

tootik is free and unencumbered software released under the terms of the [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0); see LICENSE for the license text.

The ASCII art logo at the top was made using [FIGlet](http://www.figlet.org/).
