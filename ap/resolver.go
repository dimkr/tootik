/*
Copyright 2024 - 2025 Dima Krasner

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

import (
	"context"
	"net/http"

	"github.com/dimkr/tootik/httpsig"
)

type ResolverFlag uint

const (
	// Offline disables fetching of remote actors and forces use of local or cached actors.
	Offline ResolverFlag = 1

	// InstanceActor enables discovery of the "instance actor" instead of the regular actor discovery flow.
	InstanceActor = 2

	// GroupActor makes [Resolver] prefer the first [Group] actor in the WebFinger response.
	GroupActor = 4
)

// Resolver retrieves [Actor], [Object] and [Activity] objects.
type Resolver interface {
	ResolveID(ctx context.Context, keys [2]httpsig.Key, id string, flags ResolverFlag) (*Actor, error)
	Resolve(ctx context.Context, keys [2]httpsig.Key, host, name string, flags ResolverFlag) (*Actor, error)
	Get(ctx context.Context, keys [2]httpsig.Key, url string) (*http.Response, error)
}
