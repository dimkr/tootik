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

// Package finger exposes a limited Finger interface.
package finger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text/plain"
	"log/slog"
)

func handle(ctx context.Context, domain string, cfg *cfg.Config, conn net.Conn, db *sql.DB, log *slog.Logger, wg *sync.WaitGroup) {
	if err := conn.SetDeadline(time.Now().Add(cfg.GuppyRequestTimeout)); err != nil {
		log.Warn("Failed to set deadline", "error", err)
		return
	}

	req := make([]byte, 34)
	total := 0
	for {
		n, err := conn.Read(req[total:])
		if err != nil {
			log.Warn("Failed to receive request", "error", err)
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
	log = log.With(slog.String("user", user))

	if user == "" {
		log.Warn("Invalid username specified")
		return
	}

	sep := strings.IndexByte(user, '@')
	if sep > 0 && user[sep+1:] != domain {
		log.Warn("Invalid domain specified")
		return
	} else if sep > 0 {
		user = user[:sep]
	}

	var actorString string
	if err := db.QueryRowContext(ctx, `select actor from persons where actor->>'preferredUsername' = ? and host = ?`, user, domain).Scan(&actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Info("User does not exist")
		fmt.Fprintf(conn, "Login: %s\r\nPlan:\r\nNo Plan.\r\n", user)
		return
	} else if err != nil {
		log.Warn("Failed to check if user exists", "error", err)
		return
	}

	var actor ap.Actor
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		log.Warn("Failed to unmarshal actor", "error", err)
		return
	}
	summary, links := plain.FromHTML(actor.Summary)

	posts := data.OrderedMap[string, int64]{}

	rows, err := db.QueryContext(ctx, `select object->>'content', inserted from notes where public = 1 and author = ? order by inserted desc limit 5`, actor.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Warn("Failed to query posts", "error", err)
		return
	} else if err == nil {
		for rows.Next() {
			var content string
			var inserted int64
			if err := rows.Scan(&content, &inserted); err != nil {
				log.Warn("Failed to parse post", "error", err)
				continue
			}
			posts.Store(content, inserted)
		}

		rows.Close()
	}

	fmt.Fprintf(conn, "Login: %s\r\nPlan:\r\n", user)

	for _, line := range strings.Split(summary, "\n") {
		conn.Write([]byte(line))
		conn.Write([]byte{'\r', '\n'})
	}

	links.Range(func(link, alt string) bool {
		if !strings.Contains(summary, link) {
			if alt == "" {
				conn.Write([]byte(link))
			} else {
				fmt.Fprintf(conn, "%s [%s]", link, alt)
			}
			conn.Write([]byte{'\r', '\n'})
		}

		return true
	})

	if summary != "" || len(links) > 0 {
		conn.Write([]byte{'\r', '\n'})
	}

	i := 0
	last := len(posts) - 1
	posts.Range(func(content string, inserted int64) bool {
		text, links := plain.FromHTML(content)

		conn.Write([]byte(time.Unix(inserted, 0).Format(time.DateOnly)))
		conn.Write([]byte{'\r', '\n'})
		for _, line := range strings.Split(text, "\n") {
			conn.Write([]byte(line))
			conn.Write([]byte{'\r', '\n'})
		}

		links.Range(func(link, alt string) bool {
			if !strings.Contains(text, link) {
				if alt == "" {
					conn.Write([]byte(link))
				} else {
					fmt.Fprintf(conn, "%s [%s]", link, alt)
				}
				conn.Write([]byte{'\r', '\n'})
			}

			return true
		})

		if i < last {
			conn.Write([]byte{'\r', '\n'})
		}

		i++
		return true
	})

	if len(posts) == 0 && summary == "" && len(links) == 0 {
		conn.Write([]byte("No Plan.\r\n"))
	}
}

// ListenAndServe handles Finger queries.
func ListenAndServe(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, db *sql.DB, addr string) error {
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
				log.Warn("Failed to accept a connection", "error", err)
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
			requestCtx, cancelRequest := context.WithTimeout(ctx, cfg.GuppyRequestTimeout)

			timer := time.AfterFunc(cfg.GuppyRequestTimeout, cancelRequest)

			wg.Add(1)
			go func() {
				handle(requestCtx, domain, cfg, conn, db, log, &wg)
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
