/*
Copyright 2023, 2024 Dima Krasner

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
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"io"
	"log/slog"
	"net/url"
)

// Request represents a request.
type Request struct {
	// Context specifies the request context.
	Context context.Context

	// URL specifies the requested URL.
	URL *url.URL

	// Log specifies a slog.Logger used while handling the request.
	Log *slog.Logger

	// Body optionally specifies an io.Reader to read the request body from.
	Body io.Reader

	// User optionally specifies a signed in user.
	User *ap.Actor

	// Key optionally specifies the signing key associated with User.
	Key httpsig.Key
}
