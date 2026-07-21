/*
Copyright 2026 Dima Krasner

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

package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	osuser "os/user"
	"path/filepath"
	"syscall"

	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/sqlite"
)

func main() {
	defaultDbPath := "client.sqlite3"
	if cfgDir, err := os.UserConfigDir(); err == nil {
		defaultDbPath = filepath.Join(cfgDir, "tootik-client", "client.sqlite3")
	}

	defaultUser := "user"
	if current, err := osuser.Current(); err == nil {
		defaultUser = current.Username
	}

	dbPath := flag.String("db", defaultDbPath, "database path")
	user := flag.String("user", defaultUser, "user to authenticate as")
	domain := flag.String("domain", "localhost", "server domain")
	port := flag.Int("port", 1965, "server port")

	flag.Usage = func() {
		out := flag.CommandLine.Output()

		fmt.Fprintf(out, "Usage: %s [-user USERNAME] [-domain DOMAIN] [-port PORT] [-db PATH] [PATH [INPUT]]\n\n", os.Args[0])
		fmt.Println(out, "Connects to a remote tootik or as USERNAME.")
		fmt.Println(out, "")
		flag.PrintDefaults()
		fmt.Println(out, "")
		fmt.Println(out, "USERNAME is the user to authenticate as; it defaults to OS user.")
		fmt.Println(out, "@DOMAIN is defaults to localhost and port 1965.")
		fmt.Println(out, "PATH is a Gemini path (e.g. /users, /local, /users/say).")
		fmt.Println(out, "INPUT is user input for the request.")
		fmt.Println(out, "")
		fmt.Println(out, "If stdout is a terminal, an interactive shell is started.")
		fmt.Println(out, "Otherwise, the Gemini protocol response, containing a gemtext")
		fmt.Println(out, "document, is printed to stdout.")
		fmt.Println(out, "")
		fmt.Println(out, `Gemtext is a markup format that consists of newline (\n) delimited`)
		fmt.Println(out, "lines that can be classified into 8 types according to their prefix:")
		fmt.Println(out, "heading (#), sub-heading (##), sub-sub-heading (###), link (=>), list")
		fmt.Println(out, "item (*), quote (>), preformatted block toggle (```) or text.")
		fmt.Println(out, "")
		fmt.Println(out, "Users can publish public posts, posts to followers or direct messages,")
		fmt.Println(out, "reply to, edit, delete and share posts, follow and unfollow users,")
		fmt.Println(out, "publish polls, browse local posts and hashtags, perform full-text")
		fmt.Println(out, "search and edit their profile.")
		fmt.Println(out, "")
		fmt.Println(out, "Non-existing users must register first:")
		fmt.Fprintf(out, "  %s USERNAME[@DOMAIN] /users/register generate\n", os.Args[0])
		fmt.Println(out, "")
		fmt.Println(out, "New users should read /users/help for more information.")

		os.Exit(2)
	}
	flag.Parse()

	if flag.NArg() > 2 {
		flag.Usage()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		slog.Error("Failed to create data directory", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open(sqlite.DriverName, fmt.Sprintf("%s?%s", *dbPath, sqlite.DefaultOptions))
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if _, err := db.ExecContext(
		ctx,
		`create table if not exists tofu(domain text not null primary key, hash text not null, inserted integer default (unixepoch()))`,
	); err != nil {
		slog.Error("Failed to create table", "error", err)
		os.Exit(1)
	}

	if _, err := db.ExecContext(
		ctx,
		`create table if not exists certs(address text not null primary key, cert blob not null, key blob not null, inserted integer default (unixepoch()))`,
	); err != nil {
		slog.Error("Failed to create table", "error", err)
		os.Exit(1)
	}

	if _, err := db.ExecContext(
		ctx,
		`create table if not exists redirects(source text not null primary key, target text not null, inserted integer default (unixepoch()))`,
	); err != nil {
		slog.Error("Failed to create table", "error", err)
		os.Exit(1)
	}

	if err := front.Connect(ctx, db, *user, *domain, *port, flag.Arg(0), flag.Arg(1)); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Failed to connect", "error", err)
		os.Exit(1)
	}
}
