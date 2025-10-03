# Frontend

## Gemini Frontend

* /local shows a compact list of local posts; each entry contains a link to /view.
* / is the homepage: it shows an ASCII art logo, a short description of this server and a list of local posts.
* /hashtag shows a compact list of posts with a given hashtag.
* /search shows an input prompt and redirects to /hashtag.
* /hashtags shows a list of popular hashtags.
* /fts shows an input prompt and performs full-text search in posts.
* /status shows statistics and server health metrics.

* /view shows a complete post with extra details like links in the post, a list mentioned users, a list of hashtags, a link to the author's outbox, a list of replies and a link to the parent post (if found).
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
* /users/hashtag
* /users/hashtags
* /users/fts
* /users/status
* /users/view

This way, users who prefer not to provide a client certificate when browsing to /x can reply to public posts by using /users/x instead.

To make the transition to authenticated pages more seamless, links in the user menu at the bottom of each page point to /users/x rather than /x, if the user is authenticated.

All pages follow the [subscription convention](https://gemini.circumlunar.space/docs/companion/subscription.gmi), so users can "subscribe" to a user, a hashtag, posts by followed users or other activity. This way, tootik can act as a personal fediverse aggregator. In addition, feeds like /users have separators between days, to interrupt the endless stream of incoming content, make the content consumption more intentional and prevent doomscrolling.

## Authentication

If no client certificate is provided, all pages under /users redirect the client to /users.

/users asks the client to provide a certificate. Well-behaved clients should generate a certificate, re-request /users, then reuse this certificate in future requests of /users and pages under it.

If a certificate is provided but does not belong to any user, the client is redirected to /users/register.

By default, the username associated with a client certificate is the common name specified in the certificate. If invalid or already in use by another user, /users/register asks the user to provide a different username. Once the user is registered, the client is redirected back to /users.

Once the client certificate is associated with a user, all pages under /users look up the authenticated user's data using the certificate hash.

## Posts

tootik has three kinds of posts:
* To mentioned users: posts visible to mentioned users only
* Posts: posts visible to followers of a user
* Public posts: posts visible to anyone

### Post Visibility

| Post type          | To                 | CC                              |
|--------------------|--------------------|---------------------------------|
| To mentioned users | -                  | Mentions                        |
| To followers       | Author's followers | Mentions                        |
| To public          | Public             | Mentions and author's followers |

### Reply Visibility

| Post type       | To                     | CC                                   |
|-----------------|------------------------|--------------------------------------|
| Public in To    | Post author and Public | Followers of reply author            |
| Public in CC    | Post author            | Followers of reply author and Public |
| Everything else | Post author            | Post audience                        |

## Post Editing

/users/edit cannot remove recipients from the post audience, only add more. If a post that mentions only `@a` is edited to mention only `@b`, both `a` and `b` will receive the updated post.

### Polls

tootik supports [Station](gemini://station.martinrue.com)-style polls. To publish a poll, publish a post in the form:

	[POLL post content] option 1 | option 2 | ...

For example:

	[POLL Does #tootik support polls now?] Yes | No | I don't know

Polls are multi-choice, allowed to have 2 to 5 options and end after a month.

Poll results are updated every 30m and distributed to other servers if needed.
