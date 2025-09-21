/*
Copyright 2023 - 2025 Dima Krasner

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

package front

import (
	"context"
	"io"
	"net/url"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

// Request represents a request.
type Request struct {
	// Context specifies the request context.
	Context context.Context

	// URL specifies the requested URL.
	URL *url.URL

	// Body optionally specifies an io.Reader to read the request body from.
	Body io.Reader

	// User optionally specifies a signed in user.
	User *ap.Actor

	// Keys optionally specifies the signing keys associated with User.
	Keys [2]httpsig.Key
}
