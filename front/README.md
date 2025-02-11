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
* /thread displays a tree of replies in a thread.
* /outbox shows list of posts by a user.

Users are authenticated using TLS client certificates; see [Gemini protocol specification](https://gemini.circumlunar.space/docs/specification.html) for more details. The following pages require authentication:

* /login shows posts by followed users, sorted chronologically.
* /login/mentions is like /login but shows only posts that mention the user.
* /login/register creates a new user.
* /login/follows shows a list of followed users, ordered by activity.
* /login/me redirects the user to their outbox.
* /login/resolve looks up federated user *user@domain* or local user *user*.
* /login/dm creates a post visible to mentioned users.
* /login/whisper creates a post visible to followers.
* /login/say creates a public post.
* /login/reply replies to a post.
* /login/edit edits a post.
* /login/delete deletes a post.
* /login/share shares a post.
* /login/unshare removes a shared post.
* /login/follow sends a follow request to a user.
* /login/unfollow deletes a follow request.
* /login/outbox is equivalent to /outbox but also includes a link to /login/follow or /login/unfollow.
* /login/bio allows users to edit their bio.
* /login/name allows users to set their display name.
* /login/alias allows users to set an account alias, to allow migration of accounts to tootik.
* /login/move allows users to notify followers of account migration from tootik.

Some clients generate a certificate for / (all pages of this capsule) when /foo requests a client certificate, while others use the certificate requested by /foo only for /foo and /foo/bar. Therefore, pages that don't require authentication are also mirrored under /login:

* /login/local
* /login/hashtag
* /login/hashtags
* /login/fts
* /login/status
* /login/view
* /login/thread

This way, users who prefer not to provide a client certificate when browsing to /x can reply to public posts by using /login/x instead.

To make the transition to authenticated pages more seamless, links in the user menu at the bottom of each page point to /login/x rather than /x, if the user is authenticated.

All pages follow the [subscription convention](https://gemini.circumlunar.space/docs/companion/subscription.gmi), so users can "subscribe" to a user, a hashtag, posts by followed users or other activity. This way, tootik can act as a personal fediverse aggregator. In addition, feeds like /login have separators between days, to interrupt the endless stream of incoming content, make the content consumption more intentional and prevent doomscrolling.

## Authentication

If no client certificate is provided, all pages under /login redirect the client to /login.

/login asks the client to provide a certificate. Well-behaved clients should generate a certificate, re-request /login, then reuse this certificate in future requests of /login and pages under it.

If a certificate is provided but does not belong to any user, the client is redirected to /login/register.

By default, the username associated with a client certificate is the common name specified in the certificate. If invalid or already in use by another user, /login/register asks the user to provide a different username. Once the user is registered, the client is redirected back to /login.

Once the client certificate is associated with a user, all pages under /login look up the authenticated user's data using the certificate hash.

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

/login/edit cannot remove recipients from the post audience, only add more. If a post that mentions only `@a` is edited to mention only `@b`, both `a` and `b` will receive the updated post.

### Polls

tootik supports [Station](gemini://station.martinrue.com)-style polls. To publish a poll, publish a post in the form:

	[POLL post content] option 1 | option 2 | ...

For example:

	[POLL Does #tootik support polls now?] Yes | No | I don't know

Polls are multi-choice, allowed to have 2 to 5 options and end after a month.

Poll results are updated every 30m and distributed to other servers if needed.
