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
	"github.com/dimkr/tootik/bestline"
	"github.com/dimkr/tootik/cluster"
	"github.com/dimkr/tootik/front/text"
	"golang.org/x/term"
)

const (
	cols = 80
	rows = 24
)

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

var auto = flag.Bool("auto", false, "")

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *auto {
		exe, err := os.Executable()
		if err != nil {
			panic(err)
		}

		f, _ := os.Create("demo.cast")
		defer f.Close()

		c := exec.CommandContext(ctx, exe)
		//c.Stderr = os.Stderr

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

		time.Sleep(time.Second * 10)
		cast.Down(ctx, 10)
		time.Sleep(time.Second)
		cast.PageDown()
		time.Sleep(time.Second * 3)
		cast.PageDown()
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		cast.Type(ctx, "5")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		cast.Down(ctx, 3)
		time.Sleep(time.Second * 2)
		cast.Down(ctx, 5)
		cast.Type(ctx, "q")

		cast.Type(ctx, "10")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 2)
		cast.Down(ctx, 6)
		time.Sleep(time.Second * 3)
		cast.Type(ctx, "q")

		cast.Type(ctx, "6")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 3)
		cast.Down(ctx, 5)
		time.Sleep(time.Second * 4)
		cast.Type(ctx, "q")

		cast.Type(ctx, "9")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 1)
		cast.Type(ctx, "@eve @frank Or pesto again! 🙄🙄\r")
		time.Sleep(time.Second * 2)
		cast.Down(ctx, 10)
		time.Sleep(time.Second * 3)
		cast.Type(ctx, "q")

		cast.Type(ctx, "8")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 1)
		cast.Input("@eve @frank Or pesto again! 🙄🙄")
		cast.Type(ctx, "\x08\x08\x08!! 🙄🙄🙄🙄🙄\r")
		time.Sleep(time.Second * 3)
		cast.PageDown()
		time.Sleep(time.Second * 3)
		cast.Type(ctx, "q")

		cast.Type(ctx, "17")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 5)
		cast.Type(ctx, "q")

		cast.Type(ctx, "7")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 3)
		cast.Type(ctx, "q")
		cast.Type(ctx, "2")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 3)
		cast.Down(ctx, 7)
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		cast.Type(ctx, "14")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second)
		cast.Type(ctx, "ivan@pizza.example")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 5)
		cast.PageDown()
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		cast.Type(ctx, "7")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second)
		cast.Type(ctx, "q")

		cast.Type(ctx, "2")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 5)
		cast.Type(ctx, "q")

		cast.Type(ctx, "6")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		cast.Type(ctx, "@noodles Super important question\r")
		time.Sleep(time.Second)
		cast.Down(ctx, 3)
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		cast.Type(ctx, "2")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 3)
		cast.Down(ctx, 5)
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		cast.Type(ctx, "4")
		time.Sleep(time.Second)
		cast.Type(ctx, "\r")
		time.Sleep(time.Second * 2)
		cast.Down(ctx, 10)
		time.Sleep(time.Second * 2)
		cast.Type(ctx, "q")

		time.Sleep(time.Second * 2)
		rawPty.Write([]byte{4})

		if err := c.Wait(); err != nil {
			panic(err)
		}

		if err := cast.Wait(); err != nil {
			panic(err)
		}

		return
	}

	//slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	slog.SetDefault(slog.New(slog.DiscardHandler))

	keyPairs := generateKeypairs()

	tempDir, err := os.MkdirTemp("", "tootik-demo-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	cl := seed(t{tempDir: tempDir, ctx: ctx}, keyPairs)
	defer cl.Stop()

	p := cl["pizza.example"].Handle(keyPairs["alice"], "/users")
	var history []string
	var links []string

	bestline.SetHintsCallback(func(text string, ansi1, ansi2 *string) string {
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

		/*
			for _, line := range lines {
				os.Stdout.WriteString(line)
				os.Stdout.Write([]byte{'\n'})
			}
		*/

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

		line, err := bestline.Bestlinef("\033[35m%s>\033[0m ", prompt)
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
