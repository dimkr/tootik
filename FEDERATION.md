# Federation

## Posts

tootik posts are `Note`s and polls are [Mastodon-compatible](https://docs.joinmastodon.org/spec/activitypub/#Question) `Question`s.

In addition, it supports `Page` and `Article` posts.

Different servers, frontends and clients use different HTML tags and attributes or even add extra whitespace when they construct `content` from the user's raw input, so tootik's HTML to plain text converter is only a 80/20 solution. Most posts look fine and pretty much follow the way a web frontend renders them.

tootik supports quote posts using the `quote` property proposed by [FEP-044f](https://codeberg.org/fediverse/fep/src/branch/main/fep/044f/fep-044f.md).

## Interaction Policies

tootik doesn't support interaction policies. It marks all public posts with `"automaticApproval": ["https://www.w3.org/ns/activitystreams#Public"]` and allows quoting of all public posts with this policy.

## Users

tootik users are `Person`s.

## Communities

tootik communities are `Group`s.

tootik automatically sends an `Announce` activity to followers of the community when `to` or `cc` of a post by a follower mention the community. In addition, tootik forwards the original activity but without wrapping it with an `Announce` activity like [FEP-1b12](https://codeberg.org/fediverse/fep/src/branch/main/fep/1b12/fep-1b12.md) says.

tootik's UI treats `Group` actors differently: `/outbox/$group` hides replies and sorts threads by last activity.

## HTTP Signatures

tootik implements [draft-cavage-http-signatures](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) but only partially:
* It ignores query
* It always uses `rsa-sha256` and puts `algorithm="rsa-sha256"` in outgoing requests
* It `algorithm` is specified in an incoming request, it must be `rsa-sha256` or `hs2019`
* It validates `Host`, `Date` (see `MaxRequestAge`) and `Digest`
* Validation ensures that key size is between 2048 and 8192
* Incoming `POST` requests must have at least `headers="(request-target) host date digest"`
* All other incoming requests must have at least `headers="(request-target) host date"`
* Outgoing `POST` requests have `headers="(request-target) host date content-type digest"`
* All other outgoing requests have `headers="(request-target) host date"`

In addition, tootik partially implements [RFC9421](https://datatracker.ietf.org/doc/rfc9421/):
* It supports `rsa-v1_5-sha256` and `ed25519` signatures
* If `alg` is specified, tootik validates the signature only if the key type matches `alg`
* It obeys `expires` if specified, but also validates `created` using `MaxRequestAge`
* Incoming `POST` requests must have at least `("@method" "@target-uri" "content-type" "content-digest")`
* All other incoming requests must have at least `("@method" "@target-uri")`
* If query is not empty, `@query` must be signed

tootik's actors have a traditional RSA key under `publicKey` and an Ed25519 key under `assertionMethod`, as described in [FEP-521a](https://codeberg.org/fediverse/fep/src/branch/main/fep/521a/fep-521a.md).

By default, tootik uses `draft-cavage-http-signatures` when it signs outgoing requests. It starts using RFC9421 (with Ed25519, if possible) when talking to a particular server once these capabilities are 'discovered' in one of several ways:
* When at least one actor on the server advertises support for these capabilities using [FEP-844e](https://codeberg.org/fediverse/fep/src/branch/main/fep/844e/fep-844e.md); tootik assumes this information is true although it's perfectly possible for a server to be behind a reverse proxy that drops the `Signature-Input` header
* It remembers which servers responded with `200 OK` or `202 Accepted` to a `POST` request signed with RFC9421, with or without Ed25519
* When it accepts a RFC9421-signed (with or without Ed25519) request from another server, it assumes this server also supports incoming requests signed like this
* It does **not** implement ['double-knocking'](https://swicg.github.io/activitypub-http-signature/#how-to-upgrade-supported-versions) to detect RFC9421 support, because it's uncommon and this mechanism is very likely to double the number of outgoing requests; instead, tootik randomly (see `RFC9421Threshold` and `Ed25519Threshold`) tries RFC9421 and Ed25519 in `POST` requests to servers that still haven't advertised or demonstrated support, to prevent deadlock if these servers are waiting too

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

A portable actor is created by generating or supplying a pre-generated, base58-encoded Ed25519 private key during registration. The key, like the user's `preferredUsername`, must be unique per tootik instance.

No matter if the key was generated by tootik or provided by the user, the user can recover it through the settings page.

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
    "id": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor",
    "type": "Person",
    "preferredUsername": "alice",
    "inbox": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/inbox",
    "outbox": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/outbox",
    "followers": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/followers",
    "gateways": [
        "https://a.localdomain"
    ],
    "assertionMethod": [
        {
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor",
            "id": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor#ed25519-key",
            "publicKeyMultibase": "z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8",
            "type": "Multikey"
        }
    ],
    "proof": {
        "@context": [
            "https://www.w3.org/ns/activitystreams",
            "https://w3id.org/security/data-integrity/v1",
            "https://w3id.org/security/v1"
        ],
        "created": "2025-10-04T14:19:20Z",
        "cryptosuite": "eddsa-jcs-2022",
        "proofPurpose": "assertionMethod",
        "proofValue": "z3ijraF3GjA6Rb7aY3q75KAnwm5ZEdVGsn64dRaYtkNxPhne88EcrzgtP1C1pE6CfKCtCpy338hvcttBmiudGUsbi",
        "type": "DataIntegrityProof",
        "verificationMethod": "did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8#z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8"
    },
    "publicKey": {
        "id": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor#main-key",
        "owner": "https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAs28wI+POzhRamoFp5IaK\n2zWeFtOsQYDiSuE3t9uQeqntRTNHRY4KUUYiQlvuZTxhxjLN15GpcDiRgnaR0I54\nTaPbTUHGSQkjnjvKJaZn1PltRXBlsgvibUtQHlNvGa33UMjKdFa8+IJNYJBXeoxi\nf+HKcBAMGd5hfz7xjLIaRv5usm+MHDa9tF+YDojczy2JRrxlTHq8UCrSnwp+8GxX\nleox3//Qa5S3ZMdGxk16FxJYUjd3cdC/bIkXo8UwTUm4HKi5rzoszQjnt9monYcg\nx0ldHWoYkdF386MR07irm3wvEnf9FAcolA8oMtWOWkRhyTinZWRKBuXbfpqvitdl\ndQIDAQAB\n-----END PUBLIC KEY-----\n"
    },
    "manuallyApprovesFollowers": false
}
```

Portable actors have both Ed25519 and RSA keys, allowing them to interact with actors on ActivityPub servers that don't support Ed25519 signatures.

In addition, portable actors carry an [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof, allowing other servers to securely determine which servers were "approved" by the owner of `ap://did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor`.

Moreover, all objects and activities owned by a portable actor contain an integrity proof, allowing other servers to validate their authenticity and processes them without having to fetch them from their origin first.

## Delivery

When tootik receives a `POST` request to `inbox` from a portable actor, it requires a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof generated using the actor's Ed25519 key and ability to fetch the actor, if not cached.

tootik validates the integrity proof using the Ed25519 public key extracted from the key ID, and doesn't need to fetch the actor first.

tootik's `inbox` doesn't validate HTTP signatures and simply ignores them when the sender is a portable actor. Other servers might do the same, therefore automatic detection of RFC9421 and Ed25519 support on other servers ignores `200 OK` or `202 Accepted` responses from `/.well-known/apgateway`.

tootik forwards posts by actors that share the same DID with a local actor, and replies in threads started by such actors.

## Following

Processing of `Follow`, `Undo`, `Accept` and `Reject` activities follows the 'traditional' semantics based on actor IDs: if tootik on `b.localdomain` receives a `Follow` activity for `alice@a.localdomain`, it ignores this activity because `alice@a.localdomain` is not a local actor.

tootik performs [FEP-8fcf](https://codeberg.org/fediverse/fep/src/branch/main/fep/8fcf/fep-8fcf.md) followers synchronization for portable actors, assuming that other servers track follower<>followed relationships using actor IDs and not using their canonical IDs.

## Replication

tootik forwards activites by a portable actor to all actors that share the same canonical ID, according to `gateways`.

When tootik forwards activities, it assumes that other servers use the same URL format: for example, if the `inbox` property of `alice@a.localdomain` is `https://a.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/inbox` and it forwards an activity to `bob@b.localdomain`, it sends a `POST` request to `https://b.localdomain/.well-known/apgateway/did:key:z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8/actor/inbox`.

## Limitations

* tootik does not support `ap://` identifiers, location hints and delivery to `outbox`.
* tootik assumes that activity and object IDs don't change: for example, it assumes that `Update` activities for portable posts preserve the `id` field of the original object. This matches the expectation of servers that don't support data portability and simplifies the implementation.
* tootik provides limited support for fetching of objects (like posts) and activities from `/.well-known/apgateway`: replication of data across all actors with the same canonical ID is primarily achieved using forwarding.
* The RSA key under `publicKey` is generated during registration, so different actors owned by the same DID will use different RSA keys when they talk to servers that don't support Ed25519 signatures. Therefore, servers that cache only one RSA key for two actors with the same canonical ID (which shouldn't exist) might fail to validate some signatures.
