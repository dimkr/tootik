package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log/slog"
	mrand "math/rand/v2"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"github.com/dimkr/tootik/cluster"
)

const (
	cols = 120
	rows = 30
)

var docStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("5"))

var loadingStyle = lipgloss.NewStyle().
	Faint(true).
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("241"))

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back, k.PageUp, k.PageDown, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Back},
		{k.PageUp, k.PageDown, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "page down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Back: key.NewBinding(
		key.WithKeys("backspace"),
		key.WithHelp("backspace", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	),
}

type demoModel struct {
	cluster      cluster.Cluster
	ctx          context.Context
	page         cluster.Page
	cursor       int
	url          string
	input        textinput.Model
	resized      bool
	viewport     viewport.Model
	loading      bool
	progress     progress.Model
	progressVal  float64
	loadDuration time.Duration
	loadStart    time.Time
	targetPage   cluster.Page
	seeding      bool
	spinner      spinner.Model
	history      []string
	help         help.Model
	keys         keyMap
}

type seedMsg struct {
	cluster cluster.Cluster
	page    cluster.Page
	url     string
	cert    tls.Certificate
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

func (m *demoModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		h, w := docStyle.GetFrameSize()
		if !m.resized {
			m.viewport = viewport.New(msg.Width-w, msg.Height-h-3)
			m.resized = true
		} else {
			m.viewport.Width = msg.Width - w
			m.viewport.Height = msg.Height - h - 3
		}
		m.viewport.SetContent(m.render())
	}

	if m.seeding {
		switch msg := msg.(type) {
		case seedMsg:
			m.seeding = false
			m.cluster = msg.cluster
			m.loading = true
			m.targetPage = msg.page
			m.url = msg.url
			m.progressVal = 0
			m.loadStart = time.Now()
			m.loadDuration = time.Millisecond * time.Duration(100+mrand.IntN(400))
			return m, tick()
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case tea.KeyMsg:
			if key.Matches(msg, m.keys.Quit) {
				return m, tea.Quit
			}
		}
		return m, nil
	}

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
				} else {
					m.input.Placeholder = ""
				}
				m.input.SetValue("")
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

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.loading {
				return m, tea.Batch(cmds...)
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

		case key.Matches(msg, m.keys.Down):
			if m.loading {
				return m, tea.Batch(cmds...)
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

		case key.Matches(msg, m.keys.PageDown):
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			m.viewport.PageDown()
			for i := m.viewport.YOffset; i < len(m.page.Lines) && i < m.viewport.YOffset+m.viewport.Height; i++ {
				if m.page.Lines[i].Type == cluster.Link {
					m.cursor = i
					break
				}
			}
			m.viewport.SetContent(m.render())

		case key.Matches(msg, m.keys.PageUp):
			if m.loading {
				return m, tea.Batch(cmds...)
			}

			m.viewport.PageUp()
			for i := m.viewport.YOffset; i < len(m.page.Lines) && i < m.viewport.YOffset+m.viewport.Height; i++ {
				if m.page.Lines[i].Type == cluster.Link {
					m.cursor = i
					break
				}
			}
			m.viewport.SetContent(m.render())

		case key.Matches(msg, m.keys.Enter):
			if m.loading {
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

			nextURL := m.url
			if targetCursor >= 0 && targetCursor < len(m.page.Lines) && m.page.Lines[targetCursor].Type == cluster.Link {
				nextURL = m.page.Lines[targetCursor].URL
			}

			u, err := url.Parse(nextURL)
			if err != nil {
				panic(err)
			}
			if m.input.Value() != "" {
				u.RawQuery = url.QueryEscape(m.input.Value())
			}

			m.history = append(m.history, m.url)
			m.url = u.String()

			m.targetPage = m.page.Goto(m.url)
			m.loading = true
			m.progressVal = 0
			m.loadStart = time.Now()
			m.loadDuration = time.Millisecond * time.Duration(100+mrand.IntN(400))
			return m, tea.Batch(append(cmds, tick())...)

		case key.Matches(msg, m.keys.Back):
			if m.loading || len(m.history) == 0 {
				return m, tea.Batch(cmds...)
			}

			if m.input.Placeholder != "" {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, tea.Batch(append(cmds, cmd)...)
			}

			m.url = m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]

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

func (m *demoModel) render() string {
	var s strings.Builder
	for i, l := range m.page.Lines {
		if l.Type == cluster.Heading || l.Type == cluster.SubHeading || l.Type == cluster.Link {
			s.WriteString("\033[4m")
			s.WriteString(l.Text)
			s.WriteString("\033[0m")
			if m.cursor == i {
				s.WriteString(" 👈")
			}
		} else {
			s.WriteString(l.Text)
		}

		s.WriteByte('\n')
	}
	return s.String()
}

func (m *demoModel) View() string {
	if m.seeding {
		return m.spinner.View() + "Simulating the fediverse"
	}

	var s strings.Builder
	if m.resized {
		if v := m.viewport.View(); v != "" {
			style := docStyle
			if m.loading || m.input.Placeholder != "" {
				style = loadingStyle
			}
			s.WriteString(style.Render(v) + "\n")
		}
	}

	if m.input.Placeholder != "" {
		if v := m.input.View(); v != "" {
			s.WriteString(v + "\n")
		}
	}

	if m.loading {
		if s.Len() > 0 {
			s.WriteByte('\n')
		}
		s.WriteString(m.progress.ViewAs(m.progressVal))
		s.WriteByte(' ')
		s.WriteString(m.url)
	} else {
		s.WriteString("\n\n")
	}

	if s.Len() > 0 && s.String()[s.Len()-1] != '\n' {
		s.WriteByte('\n')
	}
	s.WriteString(m.help.View(m.keys))

	return s.String()
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
		c.Stderr = os.Stderr

		rawPty, err := pty.StartWithSize(c, &pty.Winsize{Rows: rows, Cols: cols})
		if err != nil {
			panic(err)
		}
		defer rawPty.Close()

		if _, err := term.MakeRaw(rawPty.Fd()); err != nil {
			panic(err)
		}

		cast, err := startCast(rawPty, f, cols, rows)
		if err != nil {
			panic(err)
		}

		time.Sleep(time.Second * 10)

		cast.Down(ctx, 2)
		time.Sleep(time.Millisecond * 200)
		cast.Enter()

		time.Sleep(time.Millisecond * 500)
		cast.Down(ctx, 8)
		time.Sleep(time.Second * 1)
		cast.PageDown()
		time.Sleep(time.Second * 1)

		cast.Enter()
		time.Sleep(time.Second * 1)

		cast.Down(ctx, 5)
		cast.Enter()
		time.Sleep(time.Second * 1)

		cast.Down(ctx, 8)

		time.Sleep(time.Second * 1)
		cast.Enter()
		time.Sleep(time.Millisecond * 500)
		cast.Type(ctx, "@eve @frank Or pesto again! 🙄🙄\r")
		time.Sleep(time.Second * 2)

		cast.Down(ctx, 7)

		time.Sleep(time.Second * 1)
		cast.Enter()
		time.Sleep(time.Millisecond * 500)
		cast.Type(ctx, "@eve @frank Or pesto again!!! 🙄🙄🙄🙄🙄\r")
		cast.Down(ctx, 8)

		time.Sleep(time.Second * 5)

		rawPty.Write([]byte{17})
		time.Sleep(time.Second * 2)

		if err := cast.Wait(); err != nil {
			panic(err)
		}

		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	keyPairs := generateKeypairs()

	tempDir, err := os.MkdirTemp("", "tootik-demo-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	ti := textinput.New()
	ti.Focus()
	m := &demoModel{
		ctx:      ctx,
		input:    ti,
		progress: progress.New(progress.WithDefaultGradient()),
		seeding:  true,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		help:     help.New(),
		keys:     keys,
	}

	p := tea.NewProgram(m)

	done := make(chan struct{})
	go func() {
		defer func() {
			done <- struct{}{}
		}()

		cl := seed(t{tempDir: tempDir, ctx: ctx}, keyPairs)

		p.Send(seedMsg{
			cluster: cl,
			page:    cl["pizza.example"].Handle(keyPairs["alice"], "/users"),
			url:     "/users",
		})
	}()

	if _, err := p.Run(); err != nil {
		panic(err)
	}

	<-done

	if m.cluster != nil {
		m.cluster.Stop()
	}
}
