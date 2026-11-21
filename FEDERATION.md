# Federation

## Posts

tootik posts are `Note`s and polls are [Mastodon-compatible](https://docs.joinmastodon.org/spec/activitypub/#Question) `Question`s.

In addition, it supports `Page` and `Article` posts.

Different servers, frontends and clients use different HTML tags and attributes or even add extra whitespace when they construct `content` from the user's raw input, so tootik's HTML to plain text converter is only a 80/20 solution. Most posts look fine and pretty much follow the way a web frontend renders them.

tootik supports quote posts using the `quote` property proposed by [FEP-044f](https://codeberg.org/fediverse/fep/src/branch/main/fep/044f/fep-044f.md).

## Interaction Policies

tootik doesn't support interaction policies. It marks all public posts with `"automaticApproval": ["https://www.w3.org/ns/activitystreams#Public"]` and allows quoting of all public posts with this policy. If another server sends a `QuoteRequest`, tootik automatically responds with `Accept`.

## Users

tootik users are `Person`s.

## Communities

tootik communities are `Group`s and it supports mentions using the `!` prefix (for example, `!memes@example.org`) to refer to `Group`s.

tootik automatically sends an `Announce` activity to followers of the community when `to` or `cc` of a post by a follower mention the community. In addition, tootik forwards the original activity but without wrapping it with an `Announce` activity like [FEP-1b12](https://codeberg.org/fediverse/fep/src/branch/main/fep/1b12/fep-1b12.md) says.

tootik's UI treats `Group` actors differently: `/outbox/$group` hides replies and sorts threads by last activity.

## HTTP Signatures

tootik implements [draft-cavage-http-signatures](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) but only partially:
* It ignores query
* It always uses `rsa-sha256` and puts `algorithm="rsa-sha256"` in outgoing requests
* If `algorithm` is specified in an incoming request, it must be `rsa-sha256` or `hs2019`
* It validates `Host`, `Date` (see `MaxRequestAge`) and `Digest`
* Validation ensures that key size is between 2048 and 8192
* Incoming `POST` requests must have at least `headers="(request-target) host date digest"`
* All other incoming requests must have at least `headers="(request-target) host date"`
* Outgoing `POST` requests have `headers="(request-target) host date content-type digest"`
* All other outgoing requests have `headers="(request-target) host date"`

In addition, tootik partially implements [RFC9421](https://datatracker.ietf.org/doc/rfc9421/):
* It supports `rsa-v1_5-sha256`, `ed25519` and [`ml-dsa-44`](https://c2sp.org/httpsig-pq@v1.0.0) signatures
* If `alg` is specified, tootik validates the signature only if the key type matches `alg`
* It obeys `expires` if specified, but also validates `created` using `MaxRequestAge`
* Incoming `POST` requests must have at least `("@method" "@target-uri" "content-type" "content-digest")`
* All other incoming requests must have at least `("@method" "@target-uri")`
* If query is not empty, `@query` must be signed

tootik's actors have a traditional RSA key under `publicKey` and two keys under `assertionMethod` (see [FEP-521a](https://codeberg.org/fediverse/fep/src/branch/main/fep/521a/fep-521a.md)): Ed25519 and ML-DSA-44.

By default, tootik uses `draft-cavage-http-signatures` when it signs outgoing requests. It starts using RFC9421 (with Ed25519 or ML-DSA-44, if possible) when talking to a particular server once these capabilities are 'discovered' in one of several ways:
* When at least one actor on the server advertises support for these capabilities using [FEP-844e](https://codeberg.org/fediverse/fep/src/branch/main/fep/844e/fep-844e.md); tootik assumes this information is true although it's perfectly possible for a server to be behind a reverse proxy that drops the `Signature-Input` header
* It remembers which servers responded with `200 OK` or `202 Accepted` to a `POST` request signed with RFC9421, Ed25519 or ML-DSA-44
* When it accepts a RFC9421-signed (with or without Ed25519 or ML-DSA-44) request from another server, it assumes this server also supports incoming requests signed like this
* It does **not** implement ['double-knocking'](https://swicg.github.io/activitypub-http-signature/#how-to-upgrade-supported-versions) to detect RFC9421 support, because it's uncommon and this mechanism is very likely to double the number of outgoing requests; instead, tootik randomly (see `RFC9421Threshold`, `Ed25519Threshold` and `MLDSA44Threshold`) tries RFC9421, Ed25519 or ML-DSA-44 in `POST` requests to servers that still haven't advertised or demonstrated support, to prevent deadlock if these servers are waiting too

In addition, tootik randomly (see `CavageDraftFailureThreshold`) `401 Unauthorized`-rejects incoming, `draft-cavage-http-signatures`-signed `POST` requests from servers that haven't demonstrated RFC9421 support, to encourage use of RFC9421.

## Collections

tootik sets the `inbox`, `outbox` and `followers` attributes on users.

`POST` requests to `inbox` submit an activity for processing. Outgoing `GET` requests triggered by such a request (for example, to fetch the sending actor or activity from its origin) are signed using one of the the user's keys.

`POST` requests to `outbox` submit an activity for processing, but must carry a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof generated using the user's Ed25519 key.

`GET` requests to `inbox`, `outbox` and `followers` must be signed by the user, otherwise they fail with `404 Not Found`.

`inbox` returns activities delivered to the user's `inbox` and public activities delivered to any other `inbox`.

`outbox` returns activities by the user and other actors that share the same DID (see [Data Portability](#data-portability)).

`followers` returns the user's list of followers.

## Application Actor

tootik creates a special user named `actor`, which acts as an [Application Actor](https://codeberg.org/fediverse/fep/src/branch/main/fep/2677/fep-2677.md). Its key is used to sign outgoing requests not initiated by a particular user.

There are multiple ways to "discover" this actor:

1. Using [WebFinger](https://www.rfc-editor.org/rfc/rfc7033), just like any other user:

```sh
curl https://example.org/.well-known/webfinger?resource=acct:actor@example.org
```

2. For compatibility with servers that allow discovery of the Application Actor, the domain is an alias of `actor`:

```sh
curl https://example.org/.well-known/webfinger?resource=acct:example.org@example.org
```

3. [For compatibility with PieFed](https://codeberg.org/rimu/pyfedi/src/commit/452375fa17b7e5c89a5808eef168f528a47e52fe/app/activitypub/util.py#L1643), it can be fetched from / if `Accept` is `application/activity+json` or `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`:

```sh
curl -H "accept: application/activity+json" https://example.org
```

4. `/actor` returns the Application Actor, for compatibility with servers that do this and assume that other servers do the same

```sh
curl https://example.org/actor
```

5. The `links` array returned by `/.well-known/nodeinfo` links to `actor`, as [FEP-2677](https://codeberg.org/fediverse/fep/src/branch/main/fep/2677/fep-2677.md) requires

```sh
curl https://example.org/.well-known/nodeinfo
```

`actor` advertises support for RFC9421 and Ed25519 using [FEP-844e](https://codeberg.org/fediverse/fep/src/branch/main/fep/844e/fep-844e.md), to encourage other servers to use these capabilities when talking to tootik.

Before v0.21.0, tootik used to call this actor `nobody` and set the `sharedInbox` of non-portable actors to `nobody`'s inbox, to reduce the number of requests from servers that deduplicate outgoing requests by `sharedInbox` during wide delivery of posts.

## Forwarding

tootik [forwards](https://www.w3.org/TR/activitypub/#inbox-forwarding) replies (and replies to replies [...], until `MaxForwardingDepth`) to followers of the user who started the thread.

When tootik receives a forwarded activity (the sending actor belongs to different host), tootik fetches the activity from its origin. If the activity needs to be forwarded by tootik (for example: it's a forwarded `Create` activity for a reply in a thread), it forwards the received activity and not the fetched one, to let other servers decide how they want to handle this situation.

To reduce the number of outgoing requests, tootik doesn't fetch a forwarded activity from its origin if it carries a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof generated by the origin. Similarly, to reduce the number of incoming requests, tootik attaches integrity proofs to outgoing activities.

## Ghost Replies

The problem of "ghost replies" happens when tootik receives a reply for a missing post. For example, this can happen if `alice@a.localdomain` follows `bob@b.localdomain`, `bob@b.localdomain` replies to a post by `carol@c.localdomain`, but nobody on `a.localdomain` follows `carol@c.localdomain`. In this case, `b.localdomain` sends the reply to `a.localdomain` but doesn't forward its parent as well.

Every time tootik receives a **public** reply, it tries to fetch parent posts until the thread depth reaches `BackfillDepth`.

If the origin of a previously fetched post doesn't send `Update` or `Delete` activities, tootik detects edits or deletion by re-fetching that post. This re-fetch is triggered exclusively when processing a `Create` or `Update` activity for a reply (or a nested reply, up to `BackfillDepth` depth) to that post, provided it hasn't been updated within the `BackfillInterval` and no `Update` activity for it exists in the `history` table.

If `BackfillDepth` is greater than 0 and tootik receives a reply in a thread with missing or deleted intermediate replies, it may be unable to fetch all parent posts. However, if the reply includes a `context` property as defined in [FEP-f228](https://codeberg.org/fediverse/fep/src/branch/main/fep/f228/fep-f228.md) (in "collection of posts" mode), tootik uses it to fetch the root post of the thread directly.

## Account Migration

tootik supports [Mastodon's account migration mechanism](https://docs.joinmastodon.org/spec/activitypub/#Move), but ignores `Move` activities. Account migration is handled by a periodic job. If a user follows a federated user with the `movedTo` attribute set and the new account's `alsoKnownAs` attribute points back to the old account, this job sends follow requests to the new user and cancels old ones.

tootik users can set their `alsoKnownAs` field (to allow migration to tootik), or set the `movedTo` attribute and send a `Move` activity (to allow migration from tootik), through the settings page.

## Followers Synchronization

tootik supports [Mastodon's follower synchronization mechanism](https://docs.joinmastodon.org/spec/activitypub/#follower-synchronization-mechanism), also known as [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md).

tootik attaches the `Collection-Synchronization` header to outgoing activities if `to` or `cc` includes the user's followers collection.

Received `Collection-Synchronization` headers are saved in the tootik database and a periodic job (see `FollowersSyncInterval`) synchronizes the collections by sending `Undo` activities for unknown remote `Follow`s and clearing the `accepted` flag for unknown local `Follow`s.

# NodeInfo

tootik exposes instance metadata like its version number, through NodeInfo 2.0. This metadata is collected by fediverse statistics sites like [FediDB](https://fedidb.org/).

By default, tootik omits user and post counters unless `FillNodeInfoUsage` is changed to `true`.

# Data Portability

tootik partially supports [FEP-ef61](https://codeberg.org/fediverse/fep/src/branch/main/fep/ef61/fep-ef61.md) portable actors, activities and objects.

If
* `alice@a.localdomain` is `https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor`
* `bob@b.localdomain` is `https://b.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor`
* and `carol@c.localdomain` is `https://c.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor`

then tootik canonicalizes all three to `ap://did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor` and in some cases, allows one of them to operate on objects and activities "owned" by another. However, tootik is still primarily based on the 'classical mechanics' of `https://` URLs as IDs, and most "actor x is allowed to operate on object/activity y" checks are done using a strict `==` check.

Support for data portability comes into play in 5 main areas:
* Registration
* Discovery of actors
* Delivery of activities to `inbox`
* Tracking of follower<>followed relationships
* Replication of outgoing activities

## Registration

Since v0.21.0, tootik no longer offers choice between 'traditional' and portable actors: all newly registered users are portable actors.

All portable actors have both Ed25519 and ML-DSA-44 keys. By default, tootik generates both, but it allows the user to supply a base58-encoded Ed25519 or base64url-encoded ML-DSA-44 private key during registration. This key determines the DID, while the other key is generated. Like the user's `preferredUsername`, this key must be unique per tootik instance.

Note that use of ML-DSA-44 DIDs may hinder interoperability, as it produces `did:key:ukC...` DIDs (forbidden by [FEP-ef61](https://codeberg.org/fediverse/fep/src/branch/main/fep/ef61/fep-ef61.md) at the time of writing), [`mldsa44-jcs-2024`](https://www.w3.org/TR/vc-di-quantum-resistant-1.0/#cryptosuite-mldsa44-jcs-2024) integrity proofs and large objects other servers may reject.

No matter what key was used to derive the DID, the user can recover it through the settings page.

tootik does not support the [FEP-ae97](https://codeberg.org/fediverse/fep/src/branch/main/fep/ae97/fep-ae97.md) registration flow.

## Discovery

Portable actors can be looked up normally, over [WebFinger](https://www.rfc-editor.org/rfc/rfc7033):

	https://a.localdomain/.well-known/webfinger?resource=acct:alice@a.localdomain

The response points to a `https://` gateway that returns the actor object:

	{
		...
		"links": [
			{
				...
				"href": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor",
			}
		]
	}

... and the actor object uses "compatible" `https://` URLs:

```
{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        "https://w3id.org/security/data-integrity/v1",
        "https://w3id.org/security/v1"
    ],
    "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor",
    "type": "Person",
    "preferredUsername": "alice",
    "inbox": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor/inbox",
    "outbox": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor/outbox",
    "followers": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor/followers",
    "gateways": [
        "https://a.localdomain"
    ],
    "assertionMethod": [
        {
            "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor#ed25519-key",
            "type": "Multikey",
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor",
            "publicKeyMultibase": "z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL"
        },
        {
            "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor#ml-dsa-44-key",
            "type": "Multikey",
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor",
            "publicKeyMultibase": "ukCTMgzJ6Q7ojrIZbQfdXuomL9roZk1gS21Jv068UEOZyxLmi6-c7EKOrvbo4GUumwTmeKXuYVJkrFP5EHbWTsadlvrAP62Ba0EoUUAGSfyz9v22CuN5oZXdTXJwqGgC_k1A4Z0hM0Pd_DU01nHJVT0X0MeoaecVBRa2NXzn3JQ00YlJbcZ1H8U15Ofhs93VRFc-j9Bi7WWYz_Bnrij1kq7-AhLRt9uMfpp30U-4O-n33P9385gpsp1EKBxGfJr--3-wqjbJ4dfXfIfS47eOFJXKHTR6s63YcP61-Dgp38_BRbVNxfVzzfCKLCr2MzoQR6MP-rAgPYgRzAbqfplsWDZFpjYpsnc0slXRd8rq6vSr7IJ1VVsUwpfNc6Bw378rlk5a51ksXlomZu8dEXi_ehvLiIZ3q4L5Vg42LUwX5fQPhCzyQa-teXJIkVn_XFSlg7a93plgS5LTjnvKLZMF3ALjbGQjQdWpq1WI2OMVgmgtNgpU8wuveqD48D0LTBoqkPENW8dgUcO61-1h6CE-ducX9jRlgw8LZuko2sObNMlroc2R86B1jD5y7DB4NtQKD1JlSwM1_mA3Ly0mIYX1NNj5KRzQC7r9bTkTbczaDuwShO_L15YoSmN5sayQP_jTXnSxG0VokSoeypAJdsPapUJCPa72U2t6ldDlHJi8bFtdoshDJXyU-X5V-ajjqs199MClvdvuwMVFeF4G8j4YaR-P4RCoD00Q3BMFWLkQf1nKxRhUIW1l5RU7aVEr8oaD1VwvFf_H7s7yvO-l5KGDGjswsaagjCmtN9xfI7Jqq5-hPDR1rTPtDGThZmu8vlCdGGNXT3_mvtaqkYeFM2vgE9GE8-N72PSMDpwYl70MAfg_GhbkbaftM0AZy6e0gNi6hXp6kv3y-8lz1AuZvVWvhXsQ5Hqo6O4zUA_Clecf4ZPrF8H8a1gRvIDurlaql1ieR0kjc_NFR3JKbuxFHpyh21Bd9ZJ33UsYjxY7A8pqNTSq5TBNXqotq3V4260lT0MQIBkYxFVPgq1pJ3xTZ2aUPVUH8XaXBYrfNeAmb16ae2FYn38P1Fx_tjIT258F5wIlNzwq6J4ZEkUmKKfPj1r2PF5WmoW-dvHB6uF7-VjK3q2h3GqzFEvKRiG8JMRtrIvwM-ZJfdsPFoq4PES0FrdWFWGeA_9N0bATEdkJxRcpPpwQAjJBAx8q16M4dOEM0RVTnmV8f78Fm9zkTGyD7AtPCItRKHbr9zh4uFy3H9UkTqvLj0Kwdzk7d7TLyFkKuhuC06Sk-1iqHnGYoNF_JNJiF34RBc1hMl5VzKUj_qYeGvFoR-4ubX3iDWM_cUU0YYlE3PqNXQCVcs8gsbWyPqphEGMu0T7qeIZVTSQFGhMZ-k6iOG4-877zenaXiYYZ8ZHoWGhHyDY4gREcBXa35RldWmXMwJXEUmlQpRZ7phxmKn7rFhIxbr7_YCw2sJwKUFpMwHsaSAL-R54KMrggbB3KjXed-6iSUZJ7VHImG2eJTGjYcDKWmMK_03jbr6JRmuZm0gBgnHCg7XBeDDUapqt6xYWAqr0YU_qgOcBb_JeykUea4rCrywjoLY-DD4KUFNxL51G8yBOZNihnIO3nN8OZz0r4JEwZd36jeefLnwYXPuioL2lr1dcwgK9-zca7T247WlVJjEnJKyW71lo9V7CAW0kzV2ESYK-tI9KzMHFwIb9fIpyXZv6RyvkadkkPHDc7Wp0g1oeuYl70vD8UXrDDOuQX0"
        }
    ],
    "proof": {
        "@context": [
            "https://www.w3.org/ns/activitystreams",
            "https://w3id.org/security/data-integrity/v1",
            "https://w3id.org/security/v1"
        ],
        "created": "2026-08-15T06:51:20Z",
        "type": "DataIntegrityProof",
        "cryptosuite": "eddsa-jcs-2022",
        "verificationMethod": "did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL#z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL",
        "proofPurpose": "assertionMethod",
        "proofValue": "z5mwa8x2YcS6nGcBzjLPFKt4WcrbeBTyS29dooo891ommoVu4kyMCBAqceAH3PVKhUff2bgxsLCAgnKxAZGpGLUAn"
    },
    "publicKey": {
        "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor#main-key",
        "owner": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwgUdOpi0cPPCC7XYHQ+a\ngSEmMDdLArtp/tgEdNfk5gu742UWdI8+9uQqGEikjOlG+w7TSJl8ta8Foa6lQA9U\nf3O1gqHFev/jtZbArhdRSjWQuVv1nhY/WWiBP9mqIqWj4LLNoVAHXOeLZgpXDMCo\n87zbremTa+3CRCe/jGvNIzE+rC30ZerJG2o6Id2aogy/hIKP77AijZuor3/YVflR\nneXOtMloJbkY4rxKEOJZuB4iMuaExwFFReMsAif2lc8uG7j4S2kLuuqh9om7uHEW\nkhE0/6JXSXhfsqJtxVBQdaf94OdvkccHGiMUvSMPskIr0eIWhe3ioHydLlSZmV4Q\n0wIDAQAB\n-----END PUBLIC KEY-----\n"
    },
    "icon": [
        {
            "type": "Image",
            "mediaType": "image/gif",
            "url": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkm8WZrNcWpbqjJWZC3zs18P4f8cWyqaEoBmhiv5wvMUFL/actor/icon.gif"
        }
    ],
    "manuallyApprovesFollowers": false
}
```

Portable actors with DIDs derived from their ML-DSA-44 are similar but use bigger, base64url-encoded public keys:

```
{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        "https://w3id.org/security/data-integrity/v1",
        "https://w3id.org/security/v1"
    ],
    "id": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor",
    "type": "Person",
    "preferredUsername": "alice",
    "inbox": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor/inbox",
    "outbox": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor/outbox",
    "followers": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor/followers",
    "gateways": [
        "https://a.localdomain"
    ],
    "assertionMethod": [
        {
            "id": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor#ed25519-key",
            "type": "Multikey",
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor",
            "publicKeyMultibase": "z6MknwDu1csDsEsxM3cGyFtrpz9tiXdmJn7xxhvZqUQD81J5"
        },
        {
            "id": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor#ml-dsa-44-key",
            "type": "Multikey",
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor",
            "publicKeyMultibase": "ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH"
        }
    ],
    "proof": {
        "@context": [
            "https://www.w3.org/ns/activitystreams",
            "https://w3id.org/security/data-integrity/v1",
            "https://w3id.org/security/v1"
        ],
        "type": "DataIntegrityProof",
        "cryptosuite": "mldsa44-jcs-2024",
        "verificationMethod": "did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH#ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH",
        "proofPurpose": "assertionMethod",
        "proofValue": "uo_P_HJ2SgBoe9Al46O5_MGtnexWKUNHoTkk-sAVkAVqTSVjCPWQZCN6bvA-xwMG2f0e0yDrBqI-W7pdY98PW-G1fUymJ0fuHv-NVa7LHlkqlw6rk6U1yVFwrpno68kGc3230_nvcVpkJ85i_PjfBFm8XB2biUp9qzqcNPAY9MIohFaeluBd7MMpVocj61aL4QSMuA8TbYRdckVSAIaSpjYK5kq3yfWJvwFd-l6dT6Z3f8ChQPBW5E-MgNrzKbhz2eNpEmN8AcelikQINj5YZAQvrS2ZU1laOeWOkp7t4GsJooDGjGBqUnk9Yjt0zRU9VGkiaCi6LTJqko99jsA6WgIWnMK2CzD37nOkm5PWsC2LczyozYZAnhxWuG882VXLiJrkgefVLbWMtBlt-yOjen7QgdwTDVV2bTjYP0oFQYeN91PxkDxcKTKyrcM1jk-t1tPTfv69t7oQOgeFWqncJ9j3IH60MnQWUPPfszNArHcV_Nb8DNMEk-6-9w2w3iZAGR4zhjqyRLT4gzRyodBW7zqBOuucXbmGIdJsolrfn3JxAhaX5PWDd-TlB8nyGsyAbRGRXMWUpR7z3Wgfl-pv5lw7VC6GWDD9yWeZtyWF0-EIydz9nThsG7l_twpNwbS3PfVnKYRVxrDCeLfXCCv3yUx51ViCrDjryTxFCfJz-lVdW-te-_XlbZlWDrOgI2Zen0V9DnidD5wPtKxF3Pbx33NwrwIeFgn4hXwyojIgOn-rDE3J27rOc7Nm00Cj-fBOddLsh4n2qC_7NwTiHRwzHRBhpFQb3RFu9IN91ma2PsOqgYApfgm27tqRth3eqGauySsQkk7Z-gUpKT9g7-XXMgMFA1LQgoUUDV1ramWRUviUayAhWcwrd9TDH8Tl2q2cr94uVHM74CnzQ0aN4_PDqf5ipoR-6bZh85MLwgWxkyGTA3ZJ4dj4pD_MIRpFCSInPByj-kjDBpbxqh8YHoAoRUIi9HA4GTTQzRzrTzCyvjXYuIoWlE1gyAb7LqP5fH58Xik9tCLMx-KMeQQrhLTsafQV0YP5WQ69ER4kPD_Z3YXb6PFsQrRQ3RbebWM6v8L1nGsTJDq7FtG1QfKRLcbJSa7aAcTVbsw8D7JapaaRBnogwiny-3pGH4TT4eGI9oSJv3eLJjH7kDTKL66l4USm0NbauCRbJgv8LOeLOjI6FHhONFYHNrdFwaH_AiGv5AtCcG_hPUC2XlceIO4VvgslqNpOBwo4MCTqPhZker1YOr37jqZLsU4KzBpraK4rrgePi6QYi5q2iUhvyTVO1L0Zdh01j55k65DK6zYZzSIOfJ1YNWqx_WyjrE8HchuTYPP1rbCTqCyqlO5c4-umKC2gMlDh20F1_8BL-vOR7v1j4y6_ZpQR55ISVAyOP_90OFagEfXTkF1qJ32I6V2xuGNNHWmqxg39Dlg3UHUoV3BMxT9mG7KMEP6Ba66oRBzfwyoXLmwwenPkkjhgpmhrQLf0-csildhNnY0VxbJXt5kpQro6lcrsGDvTXZfdIolJFJbPU49UHK2aypAgc-wDDNz5yNJbvT4QStIVUguxc2XISQ7QAW3fr7BckEjqJWglcTnJahKcZ6R2mhAWxYfaMEjhBw0uhcfr7GtpaYizqyXCWGbemKeCkzPeMU3_q_jcrOSQfkU2TUIwsON_JU0xhkqZg55m1Ipqgx1KtiM3lmnOEY_XSX6UxB5-LYL2aGvr_MY-3F9GBG781mufjTQPha9TyBQQly_05PITS__ELeXFEsB9HexTpSVkzTPOuRDz_cYR6N56tadrVXQYMQ5212BQ5UIDXkpHOOX1XERp1Oj6ySIejN0MuqPd8JnIuFBST5AOzbbN28ZUQwV0QHHcHCC55gaXrzPay7QiAkG_3aHQa17TuBPU2twodJIoBW3wyhbRQk0wdQi_oD58P1iE-r3bouHC5PCRk14tGwBHCffEfCumJqsoaFUiB0BEY5cycYDVUHGGsuYfcIHl16XunAJBVYFWMqvuu6qe9q90xwACj7GJGBXfluKquLrlPgNDT_hm-FU8MaYaKJIFxX8Mu1zeySoVOC0E8dJqCxSuB3xCtyjuuxnegsF7FvxWkq_bi-eLc4n5F4DDz7SvEXd8jVCaBesOV8oXYsYjnuRnx6sHYdG5IrjyEPd1ylsAlkkN6ry9SL6W_K0tSlv8ytMvaLEJakTZJFdGhpzsR3_m2rdhWPUpLGsFus3UR3Dp9MMI3FdJj2rxrYSYagFLjFZIfQHA4Ts33-jVF55bKvHAvC-syMTT7WNAA0L8oLA56j57EvAB4utaZ-RlbjlM6MEA_ATEvyBVk6slyUNWzIA_4Ig26GPeV1j2cdXJ4WUoz-JArjmCwkNOoitm4ahhysF0MOEJM1JKu5gxo9WQlOIKnz38llikcje2XiLH0bJsmI-enfUxPPTatkaksK-V0wTSA4kdPBmKOnx_LDORQaqFQ26UIHA-fD56sWomSJyUHNSoWTow_uTL913qyL_87bbLhPq5Yo9iwkoSkf0NVUXiEeVteIdyLsIcuxAeQBA8ezcW9ZjPrGMYTNjqXQO0CXYJmyR0umbyTYl3IgL70B3jd8eJxSoGZaJsx4lCT6m1FJrLXQ5lVinkYD2trrS0op_RVdjTGe9fbuzdR_O9LNj68hG5O5Nfy05Jcf9btsUfonEbaniqGDMytC9SKsLSFzJazEeoEatfKpewTt7Hj9wHiRbp3dm2FZbjAwGfw8wSrkMf6uH1KpJJjbckUV48uhodIVqm71F9g6X4W5dQ3ymSdjNCe8-1d4EAFxGsj7YmJOoYob5ohLhDXljos0xz-a7oDANEE3AofdDvMAcik5ZAe_kzuUtVy6vc9NYNImQCI1sCM6UERxGiPiOmLnOhndlE704x3an7qjn-oSY0ihIq4UZKjaFDvX9E5ksQCpt1vnN2_1Jhlx7am5RBbieeFXCQ7nTDFCeT9srY36Lnf5lC_OY16hYgAC0Hg2lWmjCQ6dYNpskaVmH_MqYON7HtZJfwS7GBlu7PHLPsPSMQT5DyjAbNdS-nYLBCGp-I46WS4fXpmgAf0iE0ZOFCyY7r9TOJ_xwQX7NSux6zj4br_p4zXFTijefgTIDg5P0ZNUp-rusXH2wEMEiouQF5wdHuLlZ-jpKisu77Cw-gJFCUsOFRzeIaHi46YpKnY4QEOHiApOT5CRkhTW2WAgbi-xMXJ1t3j5vH9AA4kNU8",
        "created": "2026-08-15T06:53:38Z"
    },
    "publicKey": {
        "id": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor#main-key",
        "owner": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApYMmhHSCZFu4CiyNNoi0\nwC2GvSJjyPL4fwNlf28vbSo3yH/oM+bHRNJ44/purDzXzGb56dYljgrmbZ73UHJJ\nM+g8+2rczs2vRb/UJzzal4i1fryxAnHqHkx4cFxrVr4m/rCyq3yvebjWpMbQzBCX\n4fxu4MmPI/HqJdet1B7zR54TzCNVAJIlGT5fqFEWMcmIZNmSNa++ZQJqvr8100TC\nyh31pU/iX1Txjezh6QIHqz8NRkwFIc2PfpwOKXRloORDmZfrqQg8KgWsqstHxOAH\nbftdTZXQfHMoc9LoVp7Ll68mgvV/3xs2mjHWPPsb2HlygJRea9jp1EJINiY7oer9\newIDAQAB\n-----END PUBLIC KEY-----\n"
    },
    "icon": [
        {
            "type": "Image",
            "mediaType": "image/gif",
            "url": "https://a.localdomain/.well-known/apgateway/did:key:ukCQOcMPmypzMNCErwCrTCBhrM-Tw1mBRiynxdMEL-M6aZc5_jLrRuQtEnoRNdMPkTiLUx9kfOKHRsXffLaDvtTcajS2lLvKQmUaHM7RidMg3hAD0joyE63xALsysqQ9IMQBeZ2bCjdwQDm1FQGqssEo0KDmXvvMvIY8AkfDd4POkYbymACsu-4nx44E11pNVQoq620mWocB2U7s4ZWw5__DZST8K9PhmJBTkaVS1hnwKGkArxLtdVQHN-iEdbIya8LkqjrSE_4ogoimlyay_DNPLP--i2wke52yisa3iH7D082wZc7JTzupo_coBJ5s0RpbxslUZAtj1cUu2-KqW4lDnwVFbXElyJVWFvR_nRdCI8VC2rxvh95xelh3yy03BPbnqukUUBLeJ13jr76YhTOHbQx00CXiLZhYOtQuDoZLE7sDtd3C6KM0ajbfp5OrITVbGardPNtnFL2ABmVPVOopkE59Fcb7b6cslBUKD70wnC0XKnO5RidQx0ZNcs6gTuyxfs20VHvUrFSkIMoSU3aSuUcaLFEk2INB-hTswoT6DfoXmcFDbg9scv-kjzsmHLw8yUbQfHS5y1_7IWA0LI7oblfUAyoZSuuPi9zgHczm-TU68QzKAUCv2kf52wIFTGhJyc2Bw8raMuMB2VjHP5FtdPz-wJHNcCYV2LyJr-UoEfUPPSTdgRdMFCUQukZf4KeAqKE4qnZLes5CADd_ziYoFa-oJJwZLsN2Vp5lldvuHMk-39481QsGV6dFbJ8KdFeYHoUJ8kbS_Wy4I4boHvr7RJAZpQC3ibxk48B7vobsyL6RrZEAhMu8hEW1XFOLjb9Ye_UO3fNQDGNTudvtRu7GUl1gG6CCff0jobPrPomJxIBw9P_VJj75S9AEzIKwHNRzrXOe-TkAPp7qMQ4mwCyAhrqFFtNUi24NtclxoLWZOkSXkRO-vsxYMBVQY1VTFR5sPf1-Zno8LUNZjVf4v9Y2Ow3FrEHu2l9WfNQhA1-eGjqWTiOJSLoH_jafGYdCx6mT8GC3PpwWGz1TNwjnMVt52tAWnHsKT8KcH0dVY7ggEBCXx1rI_k_JOSlIGV55HdYg29aAyyfzNLC0oXhRsSq2Uf3xJIZtZXTGT6cACZ9ailW1UoO2jazBwVW57-W2-SKwg9Mih12Td0d4nw1Jyzqn713atCtB-UeCxzOKoRZuS6hQPwoJpoXt014Fs43_vWbKxz8ZWP0z4z4WPWLhosSo-rnZtS7v_iYCgPUqlSQSzShDb0ca8qKQMmvw52xN3hyUgCuduyE_VZ-0gj7Her-NuIA8v8WRO5zu6mhPTPZaivazH3RThyv-M1UveieXPJz3Qa85Qk-lAQ775EyNi08pjGJH4IUdN9ON_LRmQgoeIYzdR9FuFy16-qSfVyIoTffBdnE2Fcs7nuMaNwr6Ep0qjePPlXfx-fui51JkibpRzqvg5ojOecXvsu0LgIRElcr4ueG4Ed7prcVF8Lbx7V-PVTEo4rLhZCm8rXh2TUPKGoH7hNjxXS-OBgqEpRIX0DyMtODJ9_snmlfwuNrlMEACypGtgrsjOweB__idc0l4XS_Z3fRhEPA90s_culmz8-Q29h8A0PHFZFctffactceFh99Tf_Eale8All3YyzjXw6zXXFPhGKwlqXp5VeLyAho1PW-uxOOInKyrHgsHYUDQA_qjSioCVj5G4bTdV5YzV3yPnjAI2STbJ4-wfknbE16sBGVgIU_vGmOAyiwsRmPVH/actor/icon.gif"
        }
    ],
    "manuallyApprovesFollowers": false
}
```

All portable actors have RSA, Ed25519 and ML-DSA-44 keys, allowing them to interact with actors on a wide range of ActivityPub servers.

In addition, portable actors carry an [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof, allowing other servers to securely determine which servers were "approved" by the DID owner.

Moreover, all objects and activities owned by a portable actor contain an integrity proof, allowing other servers to validate their authenticity and processes them without having to fetch them from their origin first.

## Delivery

When tootik receives a `POST` request to `inbox` from a portable actor, it requires a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof generated using the private key that matches the DID, and ability to fetch the actor, if not cached.

tootik validates the integrity proof using the public key extracted from the key ID, and doesn't need to fetch the actor first.

tootik's `inbox` doesn't validate HTTP signatures and simply ignores them when the sender is a portable actor. Other servers might do the same, therefore automatic detection of RFC9421 and Ed25519 or ML-DSA-44 support on other servers ignores `200 OK` or `202 Accepted` responses from `/.well-known/apgateway`.

tootik forwards posts by actors that share the same DID with a local actor, and replies in threads started by such actors.

## Following

Processing of `Follow`, `Undo`, `Accept` and `Reject` activities follows the 'traditional' semantics based on actor IDs: if tootik on `b.localdomain` receives a `Follow` activity for `alice@a.localdomain`, it ignores this activity because `alice@a.localdomain` is not a local actor.

tootik performs [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md) followers synchronization for portable actors, assuming that other servers track follower<>followed relationships using actor IDs and not using their canonical IDs.

## Replication

tootik forwards activites by a portable actor to all actors that share the same canonical ID, according to `gateways`.

When tootik forwards activities, it assumes that other servers use the same URL format: for example, if the `inbox` property of `alice@a.localdomain` is `https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/inbox` and it forwards an activity to `bob@b.localdomain`, it sends a `POST` request to `https://b.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/inbox`.

## Limitations

* tootik does not support `ap://` identifiers and location hints.
* tootik assumes that activity and object IDs don't change: for example, it assumes that `Update` activities for portable posts preserve the `id` field of the original object. This matches the expectation of servers that don't support data portability and simplifies the implementation.
* tootik provides limited support for fetching of objects (like posts) and activities from `/.well-known/apgateway`: replication of data across all actors with the same canonical ID is primarily achieved using forwarding.
* The RSA key under `publicKey` is generated during registration, so different actors owned by the same DID will use different RSA keys when they talk to servers that don't support Ed25519 and ML-DSA-44 signatures. Therefore, servers that cache only one RSA key for two actors with the same canonical ID (which shouldn't exist) might fail to validate some signatures.
