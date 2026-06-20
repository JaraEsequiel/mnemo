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

// targetChoices is the (extensible) list of install platforms. Add a new target
// here, give it its own config step(s), and append its Options in buildResults.
var targetChoices = []labeledChoice{
	{"Local", "this machine (~/.claude, MCP user-scope)"},
	{"Cowork", "project-scoped config in a folder (cloud/sandbox)"},
}

const (
	targetLocal = iota
	targetCowork
)

var scopeChoices = []labeledChoice{
	{"user", "all your projects (recommended)"},
	{"local", "only the current project (private)"},
	{"project", "shared via .mcp.json in the project"},
}

// wizard steps.
const (
	stepTargets = iota
	stepLocalPath
	stepLocalScope
	stepCoworkPath
	stepConfirm
)

type wizard struct {
	step         int
	cursor       int          // cursor in the targets multi-select
	selected     map[int]bool // which targets are checked
	ti           textinput.Model
	scopeIdx     int
	graph        bool
	localVault   string
	coworkTarget string
	base         Options
	localDef     string
	coworkDef    string
	results      []Options
	confirmed    bool
}

func newWizard(def Options) wizard {
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 50
	cwd, _ := os.Getwd()
	coworkDef := def.Target
	if coworkDef == "" {
		coworkDef = cwd
	}
	return wizard{
		ti:        ti,
		base:      def,
		graph:     true,
		selected:  map[int]bool{targetLocal: true}, // Local pre-checked
		localDef:  def.Vault,
		coworkDef: coworkDef,
	}
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
		return w.advance()
	}

	switch w.step {
	case stepTargets:
		switch key.String() {
		case "up", "k":
			if w.cursor > 0 {
				w.cursor--
			}
		case "down", "j":
			if w.cursor < len(targetChoices)-1 {
				w.cursor++
			}
		case " ", "x":
			w.selected[w.cursor] = !w.selected[w.cursor]
		}
	case stepLocalPath, stepCoworkPath:
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	case stepLocalScope:
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
		if w.selected[targetLocal] && key.String() == "g" {
			w.graph = !w.graph
		}
	}
	return w, nil
}

func (w wizard) advance() (tea.Model, tea.Cmd) {
	switch w.step {
	case stepTargets:
		if !w.selected[targetLocal] && !w.selected[targetCowork] {
			return w, nil // require at least one
		}
		return w.goTo(stepLocalPath), nil
	case stepLocalPath:
		if strings.TrimSpace(w.ti.Value()) == "" {
			return w, nil
		}
		w.localVault = strings.TrimSpace(w.ti.Value())
		return w.goTo(stepLocalScope), nil
	case stepLocalScope:
		return w.goTo(stepCoworkPath), nil
	case stepCoworkPath:
		if strings.TrimSpace(w.ti.Value()) == "" {
			return w, nil
		}
		w.coworkTarget = strings.TrimSpace(w.ti.Value())
		return w.goTo(stepConfirm), nil
	case stepConfirm:
		w.confirmed = true
		w.results = w.buildResults()
		return w, tea.Quit
	}
	return w, nil
}

// goTo moves to step, skipping steps whose target isn't selected and seeding the
// text input where needed.
func (w wizard) goTo(step int) wizard {
	for ; step < stepConfirm; step++ {
		switch step {
		case stepLocalPath:
			if w.selected[targetLocal] {
				w.ti.SetValue(w.localDef)
				w.ti.Focus()
				w.step = step
				return w
			}
		case stepLocalScope:
			if w.selected[targetLocal] {
				w.step = step
				return w
			}
		case stepCoworkPath:
			if w.selected[targetCowork] {
				w.ti.SetValue(w.coworkDef)
				w.ti.Focus()
				w.step = step
				return w
			}
		}
	}
	w.step = stepConfirm
	return w
}

func (w wizard) buildResults() []Options {
	var out []Options
	if w.selected[targetLocal] {
		o := w.base
		o.Vault = w.localVault
		o.Scope = scopeChoices[w.scopeIdx].name
		o.WriteGraph = w.graph
		o.RegisterMCP = true
		o.Cowork = false
		o.Target = ""
		out = append(out, o)
	}
	if w.selected[targetCowork] {
		o := w.base
		o.Cowork = true
		o.Target = w.coworkTarget
		out = append(out, o)
	}
	return out
}

func (w wizard) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mnemo setup") + "\n\n")
	switch w.step {
	case stepTargets:
		b.WriteString("Install for which platform(s)?  (space to toggle, one or more)\n\n")
		for i, c := range targetChoices {
			cursor := "  "
			if i == w.cursor {
				cursor = cursorStyle.Render("▸ ")
			}
			box := "[ ]"
			if w.selected[i] {
				box = "[x]"
			}
			line := fmt.Sprintf("%s %s — %s", box, c.name, c.desc)
			if w.selected[i] {
				line = choiceStyle.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + hintStyle.Render("↑/↓ move · space toggle · enter continue · esc cancel"))
	case stepLocalPath:
		b.WriteString("Local — where should your vault live?\n")
		b.WriteString(w.ti.View() + "\n\n")
		b.WriteString(hintStyle.Render("enter to continue · esc to cancel"))
	case stepLocalScope:
		b.WriteString("Local — MCP server scope:\n\n")
		renderChoices(&b, scopeChoices, w.scopeIdx)
		b.WriteString("\n" + hintStyle.Render("↑/↓ to choose · enter to continue"))
	case stepCoworkPath:
		b.WriteString("Cowork — folder to make Cowork-ready:\n")
		b.WriteString(w.ti.View() + "\n\n")
		b.WriteString(hintStyle.Render("enter to continue · esc to cancel"))
	case stepConfirm:
		b.WriteString("Ready to install:\n\n")
		if w.selected[targetLocal] {
			graph := "yes"
			if !w.graph {
				graph = "no"
			}
			fmt.Fprintf(&b, "  Local   vault=%s · scope=%s · graph=%s %s\n",
				w.localVault, scopeChoices[w.scopeIdx].name, graph, hintStyle.Render("(g toggles graph)"))
		}
		if w.selected[targetCowork] {
			fmt.Fprintf(&b, "  Cowork  folder=%s\n", w.coworkTarget)
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

// RunWizard launches the interactive setup TUI, returning one Options per
// selected platform (Local and/or Cowork). Returns an error if cancelled.
func RunWizard(def Options) ([]Options, error) {
	m, err := tea.NewProgram(newWizard(def)).Run()
	if err != nil {
		return nil, err
	}
	w, _ := m.(wizard)
	if !w.confirmed || len(w.results) == 0 {
		return nil, fmt.Errorf("setup cancelled")
	}
	return w.results, nil
}
