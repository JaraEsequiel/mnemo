package setup

import (
	"fmt"
	"os"
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

type labeledChoice struct{ name, desc string }

var modeChoices = []labeledChoice{
	{"Local", "install into this machine (~/.claude, MCP user-scope)"},
	{"Cowork", "write project-scoped config into a folder (cloud/sandbox)"},
}

var scopeChoices = []labeledChoice{
	{"user", "all your projects (recommended)"},
	{"local", "only the current project (private)"},
	{"project", "shared via .mcp.json in the project"},
}

// wizard steps.
const (
	stepMode = iota
	stepPath // vault path (local) or target folder (cowork)
	stepScope
	stepConfirm
)

type wizard struct {
	step      int
	modeIdx   int // 0 = local, 1 = cowork
	ti        textinput.Model
	scopeIdx  int
	graph     bool
	base      Options
	localDef  string // default vault path for local
	coworkDef string // default target folder for cowork
	result    *Options
}

func newWizard(def Options) wizard {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 50
	cwd, _ := os.Getwd()
	coworkDef := def.Target
	if coworkDef == "" {
		coworkDef = cwd
	}
	return wizard{ti: ti, base: def, graph: true, localDef: def.Vault, coworkDef: coworkDef}
}

func (w *wizard) cowork() bool { return w.modeIdx == 1 }

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
		return w.advance()
	}

	switch w.step {
	case stepMode:
		switch key.String() {
		case "up", "k":
			if w.modeIdx > 0 {
				w.modeIdx--
			}
		case "down", "j":
			if w.modeIdx < len(modeChoices)-1 {
				w.modeIdx++
			}
		}
	case stepPath:
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	case stepScope:
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
	case stepConfirm:
		if !w.cowork() && key.String() == "g" {
			w.graph = !w.graph
		}
	}
	return w, nil
}

func (w wizard) advance() (tea.Model, tea.Cmd) {
	switch w.step {
	case stepMode:
		// Seed the path input with the right default for the chosen mode.
		if w.cowork() {
			w.ti.SetValue(w.coworkDef)
		} else {
			w.ti.SetValue(w.localDef)
		}
		w.step = stepPath
	case stepPath:
		if strings.TrimSpace(w.ti.Value()) == "" {
			return w, nil
		}
		if w.cowork() {
			w.step = stepConfirm // cowork has no MCP scope
		} else {
			w.step = stepScope
		}
	case stepScope:
		w.step = stepConfirm
	case stepConfirm:
		o := w.base
		if w.cowork() {
			o.Cowork = true
			o.Target = strings.TrimSpace(w.ti.Value())
		} else {
			o.Vault = strings.TrimSpace(w.ti.Value())
			o.Scope = scopeChoices[w.scopeIdx].name
			o.WriteGraph = w.graph
			o.RegisterMCP = true
		}
		w.result = &o
		return w, tea.Quit
	}
	return w, nil
}

func (w wizard) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mnemo setup") + "\n\n")
	switch w.step {
	case stepMode:
		b.WriteString("Install target:\n\n")
		renderChoices(&b, modeChoices, w.modeIdx)
		b.WriteString("\n" + hintStyle.Render("↑/↓ to choose · enter to continue · esc to cancel"))
	case stepPath:
		if w.cowork() {
			b.WriteString("Folder to make Cowork-ready:\n")
		} else {
			b.WriteString("Where should your vault live?\n")
		}
		b.WriteString(w.ti.View() + "\n\n")
		b.WriteString(hintStyle.Render("enter to continue · esc to cancel"))
	case stepScope:
		b.WriteString("MCP server scope:\n\n")
		renderChoices(&b, scopeChoices, w.scopeIdx)
		b.WriteString("\n" + hintStyle.Render("↑/↓ to choose · enter to continue"))
	case stepConfirm:
		b.WriteString("Ready to install:\n\n")
		if w.cowork() {
			fmt.Fprintf(&b, "  mode:    Cowork (project-scoped, no ~/ changes)\n")
			fmt.Fprintf(&b, "  folder:  %s\n", strings.TrimSpace(w.ti.Value()))
			fmt.Fprintf(&b, "  writes:  .mcp.json · .claude/{skills,settings,hooks} · Memory/ · .mnemo-bin/\n")
		} else {
			graph := "yes"
			if !w.graph {
				graph = "no"
			}
			fmt.Fprintf(&b, "  mode:    Local\n")
			fmt.Fprintf(&b, "  vault:   %s\n", strings.TrimSpace(w.ti.Value()))
			fmt.Fprintf(&b, "  scope:   %s\n", scopeChoices[w.scopeIdx].name)
			fmt.Fprintf(&b, "  graph:   %s  %s\n", graph, hintStyle.Render("(press g to toggle)"))
		}
		b.WriteString("\n" + hintStyle.Render("enter to install · esc to cancel"))
	}
	b.WriteString("\n")
	return b.String()
}

func renderChoices(b *strings.Builder, choices []labeledChoice, idx int) {
	for i, c := range choices {
		cursor := "  "
		line := fmt.Sprintf("%s — %s", c.name, c.desc)
		if i == idx {
			cursor = cursorStyle.Render("▸ ")
			line = choiceStyle.Render(line)
		}
		b.WriteString(cursor + line + "\n")
	}
}

// RunWizard launches the interactive setup TUI, returning the chosen Options.
// def supplies defaults (vault path, target, plugin source, skills dest).
// Returns an error if the user cancels.
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
