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
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/dimkr/tootik/cluster"
	"github.com/dimkr/tootik/front/text"
	"golang.org/x/term"
)

const (
	cols = 80
	rows = 24
)

var record = flag.String("record", "", "record to a file")

func render(p cluster.Page) ([]string, []string) {
	var lines, links []string
	linkID := 1
	for _, l := range p.Lines {
		switch l.Type {
		case cluster.Heading:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				lines = append(lines, "\033[4m# "+line+"\033[0m")
			}

		case cluster.SubHeading:
			for _, line := range text.WordWrap(l.Text, cols-3, -1) {
				lines = append(lines, "\033[4m## "+line+"\033[0m")
			}

		case cluster.Quote:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				lines = append(lines, "> "+line)
			}

		case cluster.Item:
			for i, line := range text.WordWrap(l.Text, cols-2, -1) {
				if i == 0 {
					lines = append(lines, "* "+line)
				} else {
					lines = append(lines, " "+line)
				}
			}

		case cluster.Link:
			prefix := fmt.Sprintf("[%d] ", linkID)
			for i, line := range text.WordWrap(l.Text, cols-len(prefix), -1) {
				if i == 0 {
					lines = append(lines, fmt.Sprintf("\033[4;36m[%d]\033[0;39m %s", linkID, line))
				} else {
					lines = append(lines, strings.Repeat(" ", len(prefix))+line)
				}
			}
			links = append(links, l.URL)
			linkID++

		case cluster.Preformatted:
			lines = append(lines, text.WordWrap(l.Text, cols, -1)[0])

		default:
			lines = append(lines, text.WordWrap(l.Text, cols, -1)...)
		}
	}

	return lines, links
}

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
		must(cast.Type(ctx, "@noodles Super important question\r"))
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

	p := cl["pizza.example"].Handle(keyPairs["alice"], "/users")
	var history []string
	var links []string

	bestlineSetHintsCallback(func(text string, ansi1, ansi2 *string) string {
		if text == "" && len(links) > 0 {
			*ansi1 = "\033[90m"
			*ansi2 = "\033[0m"
			return fmt.Sprintf(" 1-%d", len(links))
		} else if len(links) == 0 {
			return ""
		}

		if n, err := strconv.Atoi(text); err == nil && n > 0 {
			i := 0
			for _, line := range p.Lines {
				if line.Type != cluster.Link {
					continue
				}

				i++
				if i == n {
					*ansi1 = "\033[90m"
					*ansi2 = "\033[0m"
					return " " + line.Text
				}
			}
		}

		return ""
	})

	for {
		if err := ctx.Err(); err != nil {
			break
		}

		var lines []string
		lines, links = render(p)

		if len(lines) > 0 {
			c := exec.CommandContext(ctx, "less", "-r")
			c.Stdin = strings.NewReader(strings.Join(lines, "\n"))
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				panic(err)
			}
		}

		prompt := "pizza.example"
		if strings.HasPrefix(p.Status, "10 ") {
			prompt = p.Status[3:]
		} else {
			for _, line := range p.Lines {
				if line.Type == cluster.Heading {
					prompt = line.Text
					break
				}
			}
		}

		line, err := bestline("\033[35m%s>\033[0m ", prompt)
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if n, err := strconv.Atoi(line); err == nil && n > 0 && n <= len(links) {
			nextURL := links[n-1]
			u, err := url.Parse(nextURL)
			if err != nil {
				panic(err)
			}
			history = append(history, p.Path)
			p = p.Goto(u.String())
		} else if strings.HasPrefix(p.Status, "10 ") {
			u, err := url.Parse(p.Path)
			if err != nil {
				panic(err)
			}
			u.RawQuery = url.QueryEscape(line)
			history = append(history, p.Path)
			p = p.Goto(u.String())
		} else {
			u, err := url.Parse(line)
			if err != nil {
				fmt.Printf("Invalid URL or command: %s\n", line)
				continue
			}
			history = append(history, p.Path)
			p = p.Goto(u.String())
		}
	}
}
