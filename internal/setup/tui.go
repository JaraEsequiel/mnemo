package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	hintStyle   = lipgloss.NewStyle().Faint(true)
	choiceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
)

type scopeChoice struct{ name, desc string }

var scopeChoices = []scopeChoice{
	{"user", "all your projects (recommended)"},
	{"local", "only the current project (private)"},
	{"project", "shared via .mcp.json in the project"},
}

type wizard struct {
	step     int // 0 vault, 1 scope, 2 confirm
	ti       textinput.Model
	scopeIdx int
	graph    bool
	base     Options
	result   *Options
}

func newWizard(def Options) wizard {
	ti := textinput.New()
	ti.SetValue(def.Vault)
	ti.Placeholder = def.Vault
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 50
	return wizard{ti: ti, base: def, graph: true}
}

func (w wizard) Init() tea.Cmd { return textinput.Blink }

func (w wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	}
	switch key.String() {
	case "ctrl+c", "esc":
		return w, tea.Quit
	case "enter":
		switch w.step {
		case 0:
			if strings.TrimSpace(w.ti.Value()) == "" {
				return w, nil
			}
			w.step = 1
			return w, nil
		case 1:
			w.step = 2
			return w, nil
		case 2:
			o := w.base
			o.Vault = strings.TrimSpace(w.ti.Value())
			o.Scope = scopeChoices[w.scopeIdx].name
			o.WriteGraph = w.graph
			o.RegisterMCP = true
			w.result = &o
			return w, tea.Quit
		}
	}
	switch w.step {
	case 0:
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	case 1:
		switch key.String() {
		case "up", "k":
			if w.scopeIdx > 0 {
				w.scopeIdx--
			}
		case "down", "j":
			if w.scopeIdx < len(scopeChoices)-1 {
				w.scopeIdx++
			}
		}
	case 2:
		if key.String() == "g" {
			w.graph = !w.graph
		}
	}
	return w, nil
}

func (w wizard) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mnemo setup") + "\n\n")
	switch w.step {
	case 0:
		b.WriteString("Where should your vault live?\n")
		b.WriteString(w.ti.View() + "\n\n")
		b.WriteString(hintStyle.Render("enter to continue · esc to cancel"))
	case 1:
		b.WriteString("MCP server scope:\n\n")
		for i, s := range scopeChoices {
			cursor := "  "
			line := fmt.Sprintf("%s — %s", s.name, s.desc)
			if i == w.scopeIdx {
				cursor = cursorStyle.Render("▸ ")
				line = choiceStyle.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + hintStyle.Render("↑/↓ to choose · enter to continue"))
	case 2:
		graph := "yes"
		if !w.graph {
			graph = "no"
		}
		b.WriteString("Ready to install:\n\n")
		fmt.Fprintf(&b, "  vault:        %s\n", strings.TrimSpace(w.ti.Value()))
		fmt.Fprintf(&b, "  MCP scope:    %s\n", scopeChoices[w.scopeIdx].name)
		fmt.Fprintf(&b, "  Obsidian graph: %s  %s\n", graph, hintStyle.Render("(press g to toggle)"))
		b.WriteString("\n" + hintStyle.Render("enter to install · esc to cancel"))
	}
	b.WriteString("\n")
	return b.String()
}

// RunWizard launches the interactive setup TUI, returning the chosen Options.
// def supplies defaults (vault path, plugin source, skills dest). Returns an
// error if the user cancels.
func RunWizard(def Options) (Options, error) {
	m, err := tea.NewProgram(newWizard(def)).Run()
	if err != nil {
		return Options{}, err
	}
	w, _ := m.(wizard)
	if w.result == nil {
		return Options{}, fmt.Errorf("setup cancelled")
	}
	return *w.result, nil
}
