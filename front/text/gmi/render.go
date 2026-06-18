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

package gmi

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dimkr/tootik/front/text"
	"golang.org/x/term"
)

const fallbackCols = 80

func render(lines []Line, cols int, w io.Writer) {
	linkID := 1

	writeString := func(s string) (int, error) {
		return w.Write([]byte(s))
	}

	if sw, ok := w.(io.StringWriter); ok {
		writeString = sw.WriteString
	}

	for _, l := range lines {
		switch l.Type {
		case Heading:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				writeString("\033[4m# " + line + "\033[0m\n")
			}

		case SubHeading:
			for _, line := range text.WordWrap(l.Text, cols-3, -1) {
				writeString("\033[4m## " + line + "\033[0m\n")
			}

		case Quote:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				writeString("> " + line + "\n")
			}

		case Item:
			for i, line := range text.WordWrap(l.Text, cols-2, -1) {
				if i == 0 {
					writeString("* " + line + "\n")
				} else {
					writeString(" " + line + "\n")
				}
			}

		case Link:
			prefix := fmt.Sprintf("[%d] ", linkID)
			for i, line := range text.WordWrap(l.Text, cols-len(prefix), -1) {
				if i == 0 {
					writeString(fmt.Sprintf("\033[4;36m[%d]\033[0;39m %s\n", linkID, line))
				} else {
					writeString(strings.Repeat(" ", len(prefix)) + line + "\n")
				}
			}
			linkID++

		case Preformatted:
			writeString(text.WordWrap(l.Text, cols, -1)[0] + "\n")

		default:
			for _, line := range text.WordWrap(l.Text, cols, -1) {
				writeString(line + "\n")
			}
		}
	}
}

// Render displays a Gemtext document inside a pager.
func Render(ctx context.Context, lines []Line) error {
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols = fallbackCols
	}

	c := exec.CommandContext(ctx, "less", "-R")

	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Start(); err != nil {
		return err
	}

	render(lines, cols, stdin)
	stdin.Close()

	return c.Wait()
}
