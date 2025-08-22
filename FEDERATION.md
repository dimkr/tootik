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

## Application Actor

tootik creates a special user named `nobody`, which acts as an [Application Actor](https://codeberg.org/fediverse/fep/src/branch/main/fep/2677/fep-2677.md). Its key is used to sign outgoing requests not initiated by a particular user.

This user can be discovered using [WebFinger](https://www.rfc-editor.org/rfc/rfc7033), just like any other user:

	https://example.org/.well-known/webfinger?resource=acct:nobody@example.org

For compatibility with servers that allow discovery of the Application Actor, the domain is an alias of `nobody`:

	https://example.org/.well-known/webfinger?resource=acct:example.org@example.org

The `sharedInbox` of other users points to `nobody`'s inbox, to allow wide delivery of posts.

`nobody` advertises support for RFC9421 and Ed25519 using [FEP-844e](https://codeberg.org/fediverse/fep/src/branch/main/fep/844e/fep-844e.md), to encourage other servers to use these capabilities when talking to tootik.

## Forwarding

tootik [forwards](https://www.w3.org/TR/activitypub/#inbox-forwarding) replies (and replies to replies [...], until `MaxForwardingDepth`) to followers of the user who started the thread.

When tootik receives a forwarded activity (the sending actor belongs to different host), tootik fetches the activity from its origin. If the activity needs to be forwarded by tootik (for example: it's a forwarded `Create` activity for a reply in a thread), it forwards the received activity and not the fetched one, to let other servers decide how they want to handle this situation.

To reduce the number of outgoing requests, tootik doesn't fetch a forwarded activity from its origin if it carries a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof generated by the origin. Similarly, to reduce the number of incoming requests, tootik attaches integrity proofs to outgoing activities.

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

# Data Portability

tootik partially supports [FEP-ef61](https://codeberg.org/fediverse/fep/src/branch/main/fep/ef61/fep-ef61.md) portable actors, activities and objects.

## Registration

A portable actor is created by generating or supplying a pre-generated, base58-encoded Ed25519 private key during registration.

## Compatibility

As usual, the username is taken from the client certificate. A user named `alice` on `a.localdomain` can be looked up over [WebFinger](https://www.rfc-editor.org/rfc/rfc7033):

	https://a.localdomain/.well-known/webfinger?resource=acct:alice@a.localdomain

The response points to a `https://` gateway that returns the actor object:

	{
		...
		"links": [
			{
				...
				"href": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkjuj94k9qn7Rwddw3GnFeTq8fBcxzJ6Dgjw249LBYyqRE/actor",
			}
		]
	}

... and the actor object uses "compatible" `https://` URLs:

```
{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        "https://w3id.org/security/data-integrity/v1"
    ],
    "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor",
    "type": "Person",
    "preferredUsername": "alice",
    "inbox": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor/inbox",
    "outbox": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor/outbox",
    "followers": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor/followers",
    "gateways": [
        "https://a.localdomain"
    ],
    "assertionMethod": [
        {
            "controller": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor",
            "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor#ed25519-key",
            "publicKeyMultibase": "z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN",
            "type": "Multikey"
        }
    ],
    "proof": {
        "@context": [
            "https://www.w3.org/ns/activitystreams",
            "https://w3id.org/security/data-integrity/v1"
        ],
        "created": "2025-08-21T19:07:24Z",
        "cryptosuite": "eddsa-jcs-2022",
        "proofPurpose": "assertionMethod",
        "proofValue": "z4ykaucb9f6XZPGBM7bRPfDHPzwvyVGPedvFbiKibmgHc9fE5nyYUFa5b6Nc3YqFYL8A5L5jQ7HBVpNs14FsxsomW",
        "type": "DataIntegrityProof",
        "verificationMethod": "did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN"
    },
    "publicKey": {
        "id": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor#main-key",
        "owner": "https://a.localdomain/.well-known/apgateway/did:key:z6Mkmg7XquTdrWR7ZfUt8xADs9P4kDft9ztSZN5wq8PjuHSN/actor",
        "publicKeyPem": "-----BEGIN RSA PUBLIC KEY-----\nMIIBCgKCAQEAo1kH9SLrhJynDBwEcdzHD1wkTq96qZuj8VTMHSOh2mNcCoQap8Fw\ndlzggcXu4yQyPVg/dZTvf12xijCGOpfm6/1+4OfRL6la7FBqVDGBtmKjrjt+KEZE\ny9apO0tUEcRyvX39gbdnrX5VV/8RA4+fPD/BU6GipKhIvnBmxr/qfE9JSMlcn3YE\nSYhdjb+QryMVfs50qKtxjonHi4crkTg222qNScf7hsF31nEvrhWLkD8Pii6JPaZ+\nKp+wftNAQahYxDh0TZSKx+2ZqB8fakMqT5qNY0+ZXUk+b3FQ6XBXmf2RdMKl/HOM\nEdol4X6ZQts/qDmMx0m1hHvrxTbB6gBvcQIDAQAB\n-----END RSA PUBLIC KEY-----\n"
    },
    "manuallyApprovesFollowers": false
}
```

## Security

When tootik receives a `POST` request to `inbox` from a portable actor, it expects a valid [FEP-8b32](https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md) integrity proof and ability to fetch the actor, if not cached.

tootik validates the integrity proof using the Ed25519 public key extracted from the key ID, and doesn't need to fetch the actor first.

## Forwarding

If tootik on `a.localdomain` receives an activity from `b.localdomain` by a portable actor registered on `a.localdomain`, with gateways `a.localdomain`, `b.localdomain` and `c.localdomain`, it forwards the activity to `b.localdomain` and `c.localdomain`. In addition, tootik forwards activities forwarded by this actor: a reply in a thread started by the portable actor on `a.localdomain` will get forwarded to `b.localdomain` and `c.localdomain`.

When tootik forwards activities, it assumes that other servers use the same URL format: for example, if `did:key:x` is registered on `a.localdomain` and `b.localdomain`, `a.localdomain` puts `https://a.localdomain/.well-known/apgateway/did:key:x/actor/inbox` in `inbox` and forwards activities to `b.localdomain` by sending them to `https://b.localdomain/.well-known/apgateway/did:key:x/actor/inbox`.

If a tootik user mentions `alice@a.localdomain` in a new post and it's a portable actor that's also registered as `bob@b.localdomin`, this post is only sent to `alice@a.localdomain`: tootik assumes that the receiving server is responsible for forwarding the activity to other gateways.

## Limitations

* tootik does not support `ap://` identifiers, location hints and delivery to `outbox`.
* The RSA key under `publicKey` is generated during registration, so different actors owned by the same DID will use different RSA keys when they talk to on servers that don't support Ed25519 signatures. Therefore, servers that cache only one RSA key for a DID with two actors might fail to validate some signatures.
* Followers synchronization is disabled for portable actors, in both directions: tootik ignores the `Collection-Synchronization` header when activites are delivered to portable actors and doesn't attach it to an outgoing request if the sender is a portable actor.
