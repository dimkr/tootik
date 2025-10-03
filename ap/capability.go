/*
Copyright 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ap

// Capability is a capability that may be supported by an ActivityPub server.
type Capability uint

const (
	// CavageDraftSignatures is support for draft-cavage-http-signatures, with rsa-sha256.
	CavageDraftSignatures Capability = 1 << iota

	// RFC9421RSASignatures is support for RFC9421 HTTP signatures, with rsa-v1_5-sha256.
	RFC9421RSASignatures

	// RFC9421Ed25519Signatures is support for RFC9421 HTTP signatures, with Ed25119 keys.
	RFC9421Ed25519Signatures
)
