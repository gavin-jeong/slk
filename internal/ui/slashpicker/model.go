package slashpicker

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/gammons/slk/internal/text"
	"github.com/gammons/slk/internal/ui/styles"
)

const MaxVisible = 7

type Command struct {
	Name        string
	Description string
	UsageHint   string
}

type Result struct {
	Name string
}

type Model struct {
	commands []Command
	filtered []Command
	query    string
	selected int
	visible  bool
}

func New() Model {
	return Model{}
}

func (m *Model) SetCommands(commands []Command) {
	m.commands = commands
	if m.visible {
		m.filter()
	}
}

func (m *Model) Open() {
	m.visible = true
	m.query = ""
	m.selected = 0
	m.filter()
}

func (m *Model) Close() {
	m.visible = false
	m.query = ""
	m.selected = 0
	m.filtered = nil
}

func (m *Model) IsVisible() bool { return m.visible }

func (m *Model) SetQuery(q string) {
	m.query = q
	m.selected = 0
	m.filter()
}

func (m *Model) Query() string { return m.query }

func (m *Model) Filtered() []Command { return m.filtered }

func (m *Model) Selected() int { return m.selected }

func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *Model) MoveDown() {
	if m.selected < len(m.filtered)-1 {
		m.selected++
	}
}

func (m *Model) Select() *Result {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return nil
	}
	return &Result{Name: m.filtered[m.selected].Name}
}

func (m *Model) filter() {
	q := text.Fold(m.query)
	results := make([]Command, 0, len(m.commands))
	for _, c := range m.commands {
		name := strings.TrimPrefix(c.Name, "/")
		if q == "" || strings.HasPrefix(text.Fold(name), q) || strings.HasPrefix(text.Fold(c.Name), q) {
			results = append(results, c)
		}
	}
	if len(results) > MaxVisible {
		results = results[:MaxVisible]
	}
	m.filtered = results
}

func (m *Model) View(width int) string {
	if !m.visible || len(m.filtered) == 0 {
		return ""
	}

	var rows []string
	for i, c := range m.filtered {
		indicator := "  "
		nameStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
		metaStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
		if i == m.selected {
			indicator = lipgloss.NewStyle().Foreground(styles.Accent).Render("▌ ")
			nameStyle = nameStyle.Bold(true)
		}

		label := indicator + nameStyle.Render(c.Name)
		if c.UsageHint != "" {
			label += metaStyle.Render("  " + c.UsageHint)
		}
		rows = append(rows, label)
		if c.Description != "" {
			rows = append(rows, "  "+metaStyle.Render(c.Description))
		}
	}

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Background(styles.SurfaceDark).
		Width(width - 2).
		Render(content)
}

func FormatCommandInsert(name string) string {
	if strings.HasPrefix(name, "/") {
		return fmt.Sprintf("%s ", name)
	}
	return fmt.Sprintf("/%s ", name)
}
