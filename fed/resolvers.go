/*
Copyright 2023 Dima Krasner

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

package fed

import (
	"context"
	"golang.org/x/sync/semaphore"
	"sync"
)

const poolSize = 16

type ResolverPool struct {
	sync.Pool
	*semaphore.Weighted
}

var Resolvers = ResolverPool{
	Pool: sync.Pool{
		New: func() any {
			return &Resolver{}
		},
	},
	Weighted: semaphore.NewWeighted(poolSize),
}

func (p *ResolverPool) Borrow(ctx context.Context) (*Resolver, error) {
	if err := p.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	return p.Get().(*Resolver), nil
}

func (p *ResolverPool) Return(r *Resolver) {
	p.Put(r)
	p.Release(1)
}
