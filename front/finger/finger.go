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

package finger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/logger"
	"github.com/dimkr/tootik/text/plain"
	log "github.com/sirupsen/logrus"
)

const reqTimeout = time.Second * 30

func handle(ctx context.Context, conn net.Conn, db *sql.DB, wg *sync.WaitGroup) {
	if err := conn.SetDeadline(time.Now().Add(reqTimeout)); err != nil {
		log.WithError(err).Warn("Failed to set deadline")
		return
	}

	req := make([]byte, 34)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil {
			log.WithError(err).Warn("Failed to receive request")
			return
		}
		if n <= 0 {
			log.Warn("Failed to receive request")
			return
		}
		total += n

		if total == cap(req) {
			log.Warn("Request is too big")
			return
		}

		if total >= 2 && req[total-2] == '\r' && req[total-1] == '\n' {
			break
		}
	}

	user := string(req[:total-2])
	log := logger.New(log.Fields{"user": user})

	if user == "" {
		log.Warn("Invalid username specified")
		return
	}

	id := fmt.Sprintf("https://%s/user/%s", cfg.Domain, user)

	var exists int
	if err := db.QueryRowContext(ctx, `select exists (select 1 from persons where id = ?)`, id).Scan(&exists); err != nil {
		log.WithError(err).Warn("Failed to check if user exists")
		return
	}
	if exists == 0 {
		log.Info("User does not exist")
		fmt.Fprintf(conn, "Login: %s\nPlan:\nNo Plan.\n", user)
		return
	}

	posts := data.OrderedMap[string, int64]{}

	rows, err := db.QueryContext(ctx, `select object->>'content', inserted from notes where author = ? and ('https://www.w3.org/ns/activitystreams#Public' in (to0, to1, to2, cc0, cc1, cc2) or (to2 is not null and exists (select 1 from json_each(object->'to') where value = 'https://www.w3.org/ns/activitystreams#Public')) or (cc2 is not null and exists (select 1 from json_each(object->'cc') where value = 'https://www.w3.org/ns/activitystreams#Public'))) order by inserted desc limit 5`, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Warn("Failed to query posts")
		return
	} else if err == nil {
		for rows.Next() {
			var content string
			var inserted int64
			if err := rows.Scan(&content, &inserted); err != nil {
				log.WithError(err).Warn("Failed to parse post")
				continue
			}
			posts.Store(content, inserted)
		}

		rows.Close()
	}

	if len(posts) == 0 {
		fmt.Fprintf(conn, "Login: %s\nPlan:\nNo Plan.\n", user)
		return
	}

	fmt.Fprintf(conn, "Login: %s\nPlan:\n", user)

	i := 0
	last := len(posts) - 1
	posts.Range(func(content string, inserted int64) bool {
		text, _ := plain.FromHTML(content)

		conn.Write([]byte(time.Unix(inserted, 0).Format(time.DateOnly)))
		conn.Write([]byte{'\n'})
		conn.Write([]byte(text))
		conn.Write([]byte{'\n'})

		if i < last {
			conn.Write([]byte{'\n'})
		}

		i++
		return true
	})
}

func ListenAndServe(ctx context.Context, db *sql.DB, addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		<-ctx.Done()
		l.Close()
		wg.Done()
	}()

	conns := make(chan net.Conn)

	wg.Add(1)
	go func() {
		for ctx.Err() == nil {
			conn, err := l.Accept()
			if err != nil {
				log.WithError(err).Warn("Failed to accept a connection")
				continue
			}

			conns <- conn
		}
		wg.Done()
	}()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case conn := <-conns:
			requestCtx, cancelRequest := context.WithTimeout(ctx, reqTimeout)

			timer := time.AfterFunc(reqTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				handle(requestCtx, conn, db, &wg)
				conn.Close()
				timer.Stop()
				cancelRequest()
				wg.Done()
			}()
		}
	}

	wg.Wait()
	return nil
}
