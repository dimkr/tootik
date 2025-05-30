/*
Copyright 2024 Dima Krasner

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

// Package lock provides synchronization primitives.
package lock

import "context"

// Lock is similar to [sync.Mutex] but locking is cancellable through a [context.Context].
type Lock chan struct{}

func New() Lock {
	c := make(chan struct{}, 1)
	c <- struct{}{}
	return c
}

func (l Lock) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l:
		return nil
	}
}

func (l Lock) Unlock() {
	l <- struct{}{}
}
