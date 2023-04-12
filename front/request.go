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

package front

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	log "github.com/sirupsen/logrus"
	"net/url"
	"sync"
)

type request struct {
	Context   context.Context
	URL       *url.URL
	User      *ap.Actor
	DB        *sql.DB
	WaitGroup *sync.WaitGroup
	Log       *log.Logger
}

func (r *request) Resolve(actorID string) (*ap.Actor, error) {
	resolver, err := fed.Resolvers.Borrow(r.Context)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve %s: %w", actorID, err)
	}

	actor, err := resolver.Resolve(r.Context, r.DB, r.User, actorID)

	fed.Resolvers.Return(resolver)
	return actor, err
}

func (r *request) Send(receiver *ap.Actor, body []byte) error {
	resolver, err := fed.Resolvers.Borrow(r.Context)
	if err != nil {
		return fmt.Errorf("Failed to send a request to %s: %w", receiver.ID, err)
	}

	err = fed.Send(r.Context, r.DB, r.User, resolver, receiver, body)

	fed.Resolvers.Return(resolver)
	return err
}

func (r *request) Exec(query string, args ...any) (sql.Result, error) {
	return r.DB.ExecContext(r.Context, query, args...)
}

func (r *request) Query(query string, args ...any) (*sql.Rows, error) {
	return r.DB.QueryContext(r.Context, query, args...)
}

func (r *request) QueryRow(query string, args ...any) *sql.Row {
	return r.DB.QueryRowContext(r.Context, query, args...)
}
