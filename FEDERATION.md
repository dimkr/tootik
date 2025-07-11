# Federation

## Posts

tootik posts are `Note`s and polls are [Mastodon-compatible](https://docs.joinmastodon.org/spec/activitypub/#Question) `Question`s.

In addition, it supports `Page` and `Article` posts.

Different servers, frontends and clients use different HTML tags and attributes or even add extra whitespace when they construct `content` from the user's raw input, so tootik's HTML to plain text converter is only a 80/20 solution. Most posts look fine and pretty much follow the way a web frontend renders them.

tootik supports quote posts using the `quote` property proposed by [FEP-044f](https://codeberg.org/fediverse/fep/src/branch/main/fep/044f/fep-044f.md).

## Interaction Policies

tootik doesn't support interaction policies. It marks all public posts with `"automaticApproval": ["https://www.w3.org/ns/activitystreams#Public"]` and allows quoting of all posts with this policy.

## Users

tootik users are `Person`s.

## Communities

tootik communities are `Group`s.

tootik automatically sends an `Announce` activity to followers of the community when `to` or `cc` of a post by a follower mention the community. In addition, tootik forwards the original activity but without wrapping it with an `Announce` activity like [FEP-1b12](https://codeberg.org/fediverse/fep/src/branch/main/fep/1b12/fep-1b12.md) says.

tootik's UI treats `Group` actors differently: `/outbox/$group` hides replies and sorts threads by last activity.

## HTTP Signatures

tootik implements [draft-cavage-http-signatures-12](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) but only partially:
* It ignores query
* It always uses `rsa-sha256`, ignores `algorithm` and puts `algorithm="rsa-sha256"` in outgoing requests
* It validates `Host`, `Date` (see `MaxRequestAge`) and `Digest`
* Validation ensures that key size is between 2048 and 8192
* Incoming `POST` requests must have at least `headers="(request-target) host date digest"`
* All other incoming requests must have at least `headers="(request-target) host date"`
* Outgoing `POST` requests have `headers="(request-target) host date content-type digest"`
* All other outgoing requests have `headers="(request-target) host date"`

## Application Actor

tootik creates a special user named `nobody`, which acts as an [Application Actor](https://codeberg.org/fediverse/fep/src/branch/main/fep/2677/fep-2677.md). Its key is used to sign outgoing requests not initiated by a particular user.

This user can be discovered using [WebFinger](https://www.rfc-editor.org/rfc/rfc7033), just like any other user:

	https://example.org/.well-known/webfinger?resource=acct:nobody@example.org

For compatibility with servers that allow discovery of the Application Actor, the domain is an alias of `nobody`:

	https://example.org/.well-known/webfinger?resource=acct:example.org@example.org

The `sharedInbox` of other users points to `nobody`'s inbox, to allow wide delivery of posts.

## Forwarding

tootik [forwards](https://www.w3.org/TR/activitypub/#inbox-forwarding) replies (and replies to replies [...], until `MaxForwardingDepth`) to followers of the user who started the thread.

When tootik receives a forwarded activity (the sending actor belongs to different host), tootik fetches the activity from its origin. If the activity needs to be forwarded by tootik (for example: it's a forwarded `Create` activity for a reply in a thread), it forwards the received activity and not the fetched one, to let other servers to decide how they want to handle this situation.

tootik does not fetch missing posts to complete threads with "ghost replies".

## Outbox

tootik sets the `outbox` attribute on users, but it always leads to an empty collection.

## Account Migration

tootik supports [Mastodon's account migration mechanism](https://docs.joinmastodon.org/spec/activitypub/#Move), but ignores `Move` activities. Account migration is handled by a periodic job. If a user follows a federated user with the `movedTo` attribute set and the new account's `alsoKnownAs` attribute points back to the old account, this job sends follow requests to the new user and cancels old ones.

tootik users can set their `alsoKnownAs` field (to allow migration to tootik), or set the `movedTo` attribute and send a `Move` activity (to allow migration from tootik), through the settings page.

## Followers Synchronization

tootik supports [Mastodon's follower synchronization mechanism](https://docs.joinmastodon.org/spec/activitypub/#follower-synchronization-mechanism), also known as [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md).

tootik attaches the `Collection-Synchronization` header to outgoing activities if `to` or `cc` includes the user's followers collection.

Received `Collection-Synchronization` headers are saved in the tootik database and a periodic job (see `FollowersSyncInterval`) synchronizes the collections by sending `Undo` activities for unknown remote `Follow`s and clearing the `accepted` flag for unknown local `Follow`s.

# NodeInfo

tootik exposes instance metadata like its version number, through NodeInfo 2.0. This metadata is collected by fediverse statistics sites like [FediDB](https://fedidb.org/).

By default, tootik returns 0 in user and post counters unless `FillNodeInfoUsage` is changed to `true`.
