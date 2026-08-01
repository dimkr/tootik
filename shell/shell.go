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

// Package shell implements an interactive shell that displays the tootik UI.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/dimkr/slopline"
	"github.com/dimkr/tootik/front/text/gmi"
	"golang.org/x/term"
)

// Shell runs an interactive shell.
//
// It uses fetch to fetch the next page and follows follows redirects.
func Shell(
	ctx context.Context,
	domain string,
	u *url.URL,
	fetch func(context.Context, *url.URL) (*url.URL, string, error),
) error {
outer:
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		eff, resp, err := fetch(ctx, u)
		if err != nil {
			return err
		}
		u = eff

		status, lines, _ := gmi.Parse(resp)

		if strings.HasPrefix(status, "30 ") || strings.HasPrefix(status, "31 ") {
			rel, err := url.Parse(status[3:])
			if err != nil {
				return err
			}

			u = u.ResolveReference(rel)
			continue
		}

		if strings.HasPrefix(status, "10 ") {
			for {
				line, err := slopline.Line(fmt.Sprintf("\033[35m%s>\033[0m ", status[3:]))
				if errors.Is(err, io.EOF) {
					return nil
				} else if err != nil {
					return err
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				u.RawQuery = line
				continue outer
			}
		}

		if !term.IsTerminal(int(os.Stdout.Fd())) {
			os.Stdout.WriteString(resp)
			return nil
		}

		if err := gmi.Render(ctx, lines); err != nil {
			return err
		}

		slopline.SetHintsCallback(func(text string) (string, string, string) {
			links := 0
			for _, line := range lines {
				if line.Type == gmi.Link {
					links++
				}
			}

			if text == "" && links > 0 {
				return fmt.Sprintf(" 1-%d", links), "\033[90m", "\033[0m"
			} else if links == 0 {
				return "", "", ""
			}

			if n, err := strconv.Atoi(text); err == nil && n > 0 {
				i := 0
				for _, line := range lines {
					if line.Type != gmi.Link {
						continue
					}

					i++
					if i == n {
						return " " + line.Text, "\033[90m", "\033[0m"
					}
				}
			}

			return "", "", ""
		})

		prompt := domain
		for _, line := range lines {
			if line.Type == gmi.Heading {
				prompt = line.Text
				break
			}
		}

		line, err := slopline.Line(fmt.Sprintf("\033[35m%s>\033[0m ", prompt))
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if n, err := strconv.Atoi(line); err == nil && n > 0 {
			linkID := 1
			found := false
			for _, line := range lines {
				if line.Type != gmi.Link {
					continue
				}

				if linkID < n {
					linkID++
					continue
				}

				found = true

				rel, err := url.Parse(line.URL)
				if err != nil {
					return err
				}

				u = u.ResolveReference(rel)
				break
			}

			if !found {
				fmt.Printf("Invalid link: %s\n", line)
			}

			continue
		}

		rel, err := url.Parse(line)
		if err != nil {
			fmt.Printf("Invalid URL or command: %s\n", line)
			continue
		}

		u = u.ResolveReference(rel)
	}
}
