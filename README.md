```
   __              __  _ __  
  / /_____  ____  / /_(_) /__
 / __/ __ \/ __ \/ __/ / //_/
/ /_/ /_/ / /_/ / /_/ / ,<   
\__/\____/\____/\__/_/_/|_|  
```

## Overview

tootik is a federated nanoblogging service for the small internet.

tootik's goal is to make the fediverse lighter, more private and more accessible:
* It converts rich content into plain text and links, reducing bandwidth requirements and making content more suitable for screen readers.
* Its frontend supports Gemini, Gopher, Finger and Guppy, so an old device incapable of running a modern web browser is still good enough.
* It puts the users in control of the content they see and tries to "slow down" the fediverse to make it more compatible with the slower pace of the small internet.
* It's a single, easy-to-deploy executable that handles both the federation (using ActivityPub) and the frontend (using Gemini) aspects, while [sqlite](https://sqlite.org/) takes care of persistency.
* It should be lightweight and efficient enough to host a small community on one, cheap server, without horizontal scaling and the ongoing maintenance of a separate database.
* It implements only a small subset of ActivityPub, enough for its feature set but not more, and everything is implemented in one language ([Go](https://go.dev/)) without abstraction layers (like web or ORM frameworks), making it easy to understand and hack on.

## Building

	go generate ./migrations

Then:

	go build ./cmd/tootik

or, to build a static executable:

	go build -tags netgo,sqlite_omit_load_extension -ldflags "-linkmode external -extldflags -static" ./cmd/tootik

## Directory Structure

* cmd/ implements main().

* outbox/ translates user actions into a queue of activities that need to be sent to other servers.
* inbox/ processes a queue of activities received from other servers.
  * inbox/note/ handles insertion of posts: both posts received from other servers and posts created by local users.
* fed/ implements federation: it handles all communication with other servers, sends what's in the "outbox" queue to other servers and adds incoming activities to the "inbox" queue.
  * fed/icon/ generates pseudo-random icons used as avatars.

* front/ implements the frontend.
  * text/text/plain/ converts HTML to plain text.
  * text/text/gmi/ contains a gemtext writer.
  * text/text/gmap/ contains a gophermap writer with line wrapping.
  * text/text/guppy/ contains a Guppy response writer.
  * front/gemini/ exposes the frontend over Gemini.
  * front/gopher/ exposes the frontend over Gopher.
  * front/finger/ exposes some content over Finger.
  * front/guppy/ exposes the frontend over Guppy.

* ap/ implements ActivityPub vocabulary.
* migrations/ contains the database schema.
* data/ contains useful data structures.
* cfg/ contains global configuration parameters.

* test/ contains tests.

## Gemini Frontend

* /local shows a compact list of local posts; each entry contains a link to /view.
* / is the homepage: it shows an ASCII art logo, a short description of this server and a list of local posts.
* /federated shows a compact list of federated posts.
* /hashtag shows a compact list of posts with a given hashtag.
* /search shows an input prompt and redirects to /hashtag.
* /hashtags shows a list of popular hashtags.
* /stats shows statistics and server health metrics.

* /view shows a complete post with extra details like links in the post, a list mentioned users, a list of hashtags, a link to the author's outbox, a list of replies and a link to the parent post (if found).
* /thread displays a tree of replies in a thread.
* /outbox shows list of posts by a user.

Users are authenticated using TLS client certificates; see [Gemini protocol specification](https://gemini.circumlunar.space/docs/specification.html) for more details. The following pages require authentication:

* /users shows the number of incoming posts by date.
* /users/inbox shows a list of posts by followed users and posts sent to the authenticated user.
* /users/register creates a new user.
* /users/follows shows a list of followed users, ordered by activity.
* /users/resolve looks up federated user *user@domain* or local user *user*.
* /users/dm creates a post visible to a given user.
* /users/whisper creates a post visible to followers.
* /users/say creates a public post.
* /users/reply replies to a post.
* /users/edit edits a post.
* /users/delete deletes a post.
* /users/follow sends a follow request to a user.
* /users/unfollow deletes a follow request.
* /users/outbox is equivalent to /outbox but also includes a link to /users/follow or /users/unfollow.

Some clients generate a certificate for / (all pages of this capsule) when /foo requests a client certificate, while others use the certificate requested by /foo only for /foo and /foo/bar. Therefore, pages that don't require authentication are also mirrored under /users:

* /users/local
* /users/federated
* /users/hashtag
* /users/hashtags
* /users/stats
* /users/view
* /users/thread

This way, users who prefer not to provide a client certificate when browsing to /x can reply to public posts by using /users/x instead.

To make the transition to authenticated pages more seamless, links in the user menu at the bottom of each page point to /users/x rather than /x, if the user is authenticated.

All pages follow the [subscription convention](https://gemini.circumlunar.space/docs/companion/subscription.gmi), so users can "subscribe" to a user, a hashtag, posts by followed users or other activity. This way, tootik can act as a personal, curated and prioritized fediverse aggregator. In addition, /users/inbox always shows posts received within a given day, to interrupt the endless stream of incoming content, make the content consumption more intentional and prevent doomscrolling.

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

| Post type   | To                 | CC                              |
|-------------|--------------------|---------------------------------|
| Message     | Receiving user     | -                               |
| Post        | Author's followers | Mentions                        |
| Public post | Public             | Mentions and author's followers |

### Reply Visibility

| Post type   | To          | CC                                                              |
|-------------|-------------|-----------------------------------------------------------------|
| Message     | Post author | -                                                               |
| Post        | Post author | Post recipients, mentions and followers of reply author         |
| Public post | Post author | Post recipients, mentions, followers of reply author and Public |

### Post Editing

/users/edit only changes the content and the last update timestamp of a post. It does **not** change the post audience and mentioned users.

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
