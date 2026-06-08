package gemtext

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dimkr/tootik/front/text"
)

func render(lines []Line, cols int, w io.Writer) {
	linkID := 1

	for _, l := range lines {
		switch l.Type {
		case Heading:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				io.WriteString(w, "\033[4m# "+line+"\033[0m\n")
			}

		case SubHeading:
			for _, line := range text.WordWrap(l.Text, cols-3, -1) {
				io.WriteString(w, "\033[4m## "+line+"\033[0m\n")
			}

		case Quote:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				io.WriteString(w, "> "+line+"\n")
			}

		case Item:
			for i, line := range text.WordWrap(l.Text, cols-2, -1) {
				if i == 0 {
					io.WriteString(w, "* "+line+"\n")
				} else {
					io.WriteString(w, " "+line+"\n")
				}
			}

		case Link:
			prefix := fmt.Sprintf("[%d] ", linkID)
			for i, line := range text.WordWrap(l.Text, cols-len(prefix), -1) {
				if i == 0 {
					io.WriteString(w, fmt.Sprintf("\033[4;36m[%d]\033[0;39m %s\n", linkID, line))
				} else {
					io.WriteString(w, strings.Repeat(" ", len(prefix))+line+"\n")
				}
			}
			linkID++

		case Preformatted:
			io.WriteString(w, text.WordWrap(l.Text, cols, -1)[0]+"\n")

		default:
			for _, line := range text.WordWrap(l.Text, cols, -1) {
				io.WriteString(w, line+"\n")
			}
		}
	}
}

func Pager(ctx context.Context, lines []Line, cols int) error {
	c := exec.CommandContext(ctx, "less", "-r")

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
