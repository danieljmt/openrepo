// Package picker provides the interactive fuzzy repo selector.
package picker

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/danieljmt/openrepo/internal/index"
	"github.com/danieljmt/openrepo/internal/match"
)

const maxVisible = 12

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	nameStyle     = lipgloss.NewStyle()
	selNameStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	noMatchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
)

type model struct {
	input    textinput.Model
	repos    []index.Repo
	stats    match.StatsFunc
	filtered []index.Repo
	cursor   int
	offset   int
	choice   *index.Repo
	aborted  bool
}

func newModel(repos []index.Repo, query string, stats match.StatsFunc) model {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("> ")
	ti.SetValue(query)
	ti.Focus()
	m := model{input: ti, repos: repos, stats: stats}
	m.refilter()
	return m
}

func (m *model) refilter() {
	m.filtered = match.RankWithFallback(m.repos, m.input.Value(), m.stats)
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
		m.offset = 0
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.aborted = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				r := m.filtered[m.cursor]
				m.choice = &r
			}
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlK:
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil
		case tea.KeyDown, tea.KeyCtrlJ:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+maxVisible {
					m.offset = m.cursor - maxVisible + 1
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()
	return m, cmd
}

func (m model) View() string {
	s := m.input.View() + "\n"
	if len(m.filtered) == 0 {
		return s + noMatchStyle.Render("  no matching repos") + "\n"
	}
	end := min(m.offset+maxVisible, len(m.filtered))
	for i := m.offset; i < end; i++ {
		r := m.filtered[i]
		cursor, name := "  ", nameStyle.Render(r.Name)
		if i == m.cursor {
			cursor = cursorStyle.Render("❯ ")
			name = selNameStyle.Render(r.Name)
		}
		line := cursor + name + dimStyle.Render("  "+r.Host+"/"+r.Org)
		if c := m.stats(r.Path).Count; c > 0 {
			line += countStyle.Render(fmt.Sprintf("  ×%d", c))
		}
		s += line + "\n"
	}
	if extra := len(m.filtered) - end; extra > 0 {
		s += dimStyle.Render(fmt.Sprintf("  … %d more", extra)) + "\n"
	}
	return s
}

// Pick runs the interactive picker over repos, with query pre-typed. It
// renders to stderr and reads the TTY directly, so it works inside command
// substitution. Returns false if the user cancelled.
func Pick(repos []index.Repo, query string, stats match.StatsFunc) (index.Repo, bool, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return index.Repo{}, false, fmt.Errorf("picker needs a terminal: %w", err)
	}
	defer tty.Close()

	p := tea.NewProgram(newModel(repos, query, stats),
		tea.WithInput(tty), tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return index.Repo{}, false, err
	}
	m := final.(model)
	if m.aborted || m.choice == nil {
		return index.Repo{}, false, nil
	}
	return *m.choice, true, nil
}
