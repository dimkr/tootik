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
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const (
	cols = 80
	rows = 24
)

var record = flag.String("record", "", "record to a file")

func delay(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
		panic(ctx.Err())
	case <-time.After(d):
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *record != "" {
		exe, err := os.Executable()
		if err != nil {
			panic(err)
		}

		filename := *record
		if filename == "" {
			filename = "demo.cast"
		}

		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		c := exec.CommandContext(ctx, exe)

		rawPty, err := pty.StartWithSize(c, &pty.Winsize{Rows: rows, Cols: cols})
		if err != nil {
			panic(err)
		}
		defer rawPty.Close()

		if _, err := term.MakeRaw(int(rawPty.Fd())); err != nil {
			panic(err)
		}

		cast, err := startCast(rawPty, f, cols, rows)
		if err != nil {
			panic(err)
		}

		delay(ctx, time.Second*10)
		must(cast.Down(ctx, 10))
		delay(ctx, time.Second)
		must(cast.PageDown())
		delay(ctx, time.Second*3)
		must(cast.PageDown())
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Input("5"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		must(cast.Down(ctx, 3))
		delay(ctx, time.Second*2)
		must(cast.Down(ctx, 5))
		must(cast.Input("q"))

		must(cast.Type(ctx, "10"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*2)
		must(cast.Down(ctx, 6))
		delay(ctx, time.Second*3)
		must(cast.Input("q"))

		must(cast.Input("6"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*3)
		must(cast.Down(ctx, 5))
		delay(ctx, time.Second*1)
		must(cast.Input("q"))

		must(cast.Input("9"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*1)
		must(cast.Type(ctx, "@eve @frank Or pesto again! 🙄🙄\r"))
		delay(ctx, time.Second*2)
		must(cast.Down(ctx, 10))
		delay(ctx, time.Second*3)
		must(cast.Input("q"))

		must(cast.Input("8"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*1)
		must(cast.Input("@eve @frank Or pesto again! 🙄🙄"))
		must(cast.Type(ctx, "\x08\x08\x08!! 🙄🙄🙄🙄🙄\r"))
		delay(ctx, time.Second*3)
		must(cast.PageDown())
		delay(ctx, time.Second*3)
		must(cast.Input("q"))

		must(cast.Type(ctx, "17"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*5)
		must(cast.Input("q"))

		must(cast.Input("7"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*3)
		must(cast.Input("q"))
		must(cast.Input("2"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*3)
		must(cast.Down(ctx, 5))
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Type(ctx, "14"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second)
		must(cast.Type(ctx, "ivan@pizza.example"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*5)
		must(cast.PageDown())
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Input("7"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second)
		must(cast.Input("q"))

		must(cast.Input("2"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*5)
		must(cast.Input("q"))

		must(cast.Input("6"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		must(cast.Type(ctx, "!noodles Super important question\r"))
		delay(ctx, time.Second)
		must(cast.Down(ctx, 3))
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Input("2"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*3)
		must(cast.Down(ctx, 5))
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Input("4"))
		delay(ctx, time.Second)
		must(cast.Input("\r"))
		delay(ctx, time.Second*2)
		must(cast.Down(ctx, 10))
		delay(ctx, time.Second*2)
		must(cast.Input("q"))

		must(cast.Type(ctx, "12"))
		delay(ctx, time.Second)
		must(cast.Backspace())
		must(cast.Backspace())
		must(cast.CtrlD())

		if err := c.Wait(); err != nil {
			panic(err)
		}

		if err := cast.Wait(); err != nil {
			panic(err)
		}

		return
	}

	slog.SetDefault(slog.New(slog.DiscardHandler))

	tempDir, err := os.MkdirTemp("", "tootik-demo-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	keyPairs := generateKeypairs()

	cl := seed(t{tempDir: tempDir, ctx: ctx}, keyPairs)
	defer cl.Stop()

	if err := cl["pizza.example"].Frontend.Handler.Shell(
		ctx,
		"alice",
		"pizza.example",
	); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}
