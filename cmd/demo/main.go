package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	mrand "math/rand/v2"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dimkr/tootik/cluster"
)

type demoModel struct {
	ctx          context.Context
	page         cluster.Page
	cursor       int
	url          string
	cert         tls.Certificate
	input        textarea.Model
	viewport     viewport.Model
	loading      bool
	progress     progress.Model
	progressVal  float64
	loadDuration time.Duration
	loadStart    time.Time
	targetPage   cluster.Page
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*10, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func findFirstLink(page cluster.Page) int {
	for i, l := range page.Lines {
		if l.Type == cluster.Link {
			return i
		}
	}
	return -1
}

func (m demoModel) Init() tea.Cmd {
	return nil
}

func (m demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.loading {
		var cmd tea.Cmd
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tickMsg:
		if m.loading {
			elapsed := time.Since(m.loadStart)
			if elapsed >= m.loadDuration {
				m.loading = false
				m.page = m.targetPage
				if strings.HasPrefix(m.page.Status, "10 ") {
					m.input.Placeholder = m.page.Status[3:]
					m.input.SetValue("")
				} else {
					m.input.Placeholder = ""
				}
				m.cursor = findFirstLink(m.page)
				m.viewport.SetContent(m.render())
				m.viewport.SetYOffset(0)
			} else {
				totalLen := 0
				for _, l := range m.targetPage.Lines {
					totalLen += len(l.Text) + 1
				}

				if totalLen == 0 {
					m.progressVal = float64(elapsed) / float64(m.loadDuration)
				} else {
					targetLen := int(float64(totalLen) * float64(elapsed) / float64(m.loadDuration))
					currentLen := 0
					for _, l := range m.targetPage.Lines {
						lineLen := len(l.Text) + 1
						if currentLen+lineLen > targetLen {
							break
						}
						currentLen += lineLen
					}
					m.progressVal = float64(currentLen) / float64(totalLen)
				}
				return m, tea.Batch(append(cmds, tick())...)
			}
		}

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 2
		m.viewport.SetContent(m.render())

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "up":
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			if m.input.Placeholder != "" {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, tea.Batch(append(cmds, cmd)...)
			}

			oldCursor := m.cursor
			for i := m.cursor - 1; i >= 0; i-- {
				if m.page.Lines[i].Type == cluster.Link {
					m.cursor = i
					break
				}
			}
			if m.cursor == oldCursor {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.cursor < m.viewport.YOffset {
				m.viewport.SetYOffset(m.cursor)
			}
			m.viewport.SetContent(m.render())

		case "down":
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			if m.input.Placeholder != "" {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, tea.Batch(append(cmds, cmd)...)
			}

			oldCursor := m.cursor
			for i := m.cursor + 1; i < len(m.page.Lines); i++ {
				if m.page.Lines[i].Type == cluster.Link {
					m.cursor = i
					break
				}
			}
			if m.cursor == oldCursor {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
				m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
			}
			m.viewport.SetContent(m.render())

		case "enter", "ctrl+d":
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			if msg.String() == "enter" && m.input.Placeholder != "" {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, tea.Batch(append(cmds, cmd)...)
			}

			if msg.String() == "ctrl+d" && m.input.Placeholder == "" {
				return m, tea.Batch(cmds...)
			}

			// If cursor is set and visible, use it. Otherwise, find the first link from the top of the viewport.
			targetCursor := m.cursor
			if targetCursor < m.viewport.YOffset || targetCursor >= m.viewport.YOffset+m.viewport.Height {
				targetCursor = -1
				for i := m.viewport.YOffset; i < len(m.page.Lines) && i < m.viewport.YOffset+m.viewport.Height; i++ {
					if m.page.Lines[i].Type == cluster.Link {
						targetCursor = i
						break
					}
				}
			}

			if targetCursor >= 0 && targetCursor < len(m.page.Lines) && m.page.Lines[targetCursor].Type == cluster.Link {
				m.url = m.page.Lines[targetCursor].URL
			}

			u, err := url.Parse(m.url)
			if err != nil {
				panic(err)
			}
			if m.input.Value() != "" {
				u.RawQuery = url.QueryEscape(m.input.Value())
			}
			m.url = u.String()

			m.targetPage = m.page.Goto(m.url)
			m.loading = true
			m.progressVal = 0
			m.loadStart = time.Now()
			m.loadDuration = time.Millisecond * time.Duration(100+mrand.IntN(400))
			return m, tea.Batch(append(cmds, tick())...)

		default:
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			if m.input.Placeholder != "" {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func (m demoModel) render() string {
	var s strings.Builder
	for i, l := range m.page.Lines {
		if m.cursor == i {
			s.WriteString("\033[1;4m")
			s.WriteString(l.Text)
			s.WriteString("\033[0m\n")
		} else if l.Type == cluster.Heading || l.Type == cluster.SubHeading || l.Type == cluster.Link {
			s.WriteString("\033[4m")
			s.WriteString(l.Text)
			s.WriteString("\033[0m\n")
		} else {
			s.WriteString(l.Text)
			s.WriteByte('\n')
		}
	}
	return s.String()
}

func (m demoModel) View() string {
	var s strings.Builder
	if v := m.viewport.View(); v != "" {
		s.WriteString(v + "\n")
	}

	if m.input.Placeholder != "" {
		if v := m.input.View(); v != "" {
			if s.Len() > 0 {
				s.WriteByte('\n')
			}
			s.WriteString(v + "\n")
		}
	}

	if m.loading {
		if s.Len() > 0 {
			s.WriteByte('\n')
		}
		s.WriteString(m.progress.ViewAs(m.progressVal))
	}

	return s.String()
}

func main() {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	keyPairs := generateKeypairs()

	tempDir, err := os.MkdirTemp("", "tootik-demo-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cluster := seed(t{tempDir: tempDir, ctx: ctx}, keyPairs)
	defer cluster.Stop()

	aliceKeyPair := keyPairs["alice"]
	page := cluster["pizza.example"].Handle(aliceKeyPair, "/")
	ti := textarea.New()
	ti.SetWidth(80)
	ti.SetHeight(3)
	ti.Focus()
	vp := viewport.New(80, 20)
	m := demoModel{
		ctx:      ctx,
		page:     page,
		url:      "gemini://pizza.example/",
		cursor:   findFirstLink(page),
		cert:     aliceKeyPair,
		input:    ti,
		viewport: vp,
		progress: progress.New(progress.WithDefaultGradient()),
	}
	m.viewport.SetContent(m.render())
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
