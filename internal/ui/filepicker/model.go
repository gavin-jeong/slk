package filepicker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/gammons/slk/internal/text"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/overlay"
	"github.com/gammons/slk/internal/ui/styles"
	"github.com/muesli/reflow/truncate"
)

type Result struct {
	Path string
}

type entry struct {
	Name     string
	Path     string
	Dir      bool
	Parent   bool
	Size     int64
}

type Model struct {
	visible  bool
	cwd      string
	query    string
	entries  []entry
	filtered []int
	selected int
	err      string
}

func New() Model { return Model{} }

func (m *Model) Open() {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd, _ = os.UserHomeDir()
	}
	m.OpenAt(cwd)
}

func (m *Model) OpenAt(path string) {
	m.visible = true
	m.query = ""
	m.selected = 0
	m.setDir(path)
}

func (m *Model) Close() {
	m.visible = false
	m.query = ""
	m.selected = 0
	m.entries = nil
	m.filtered = nil
	m.err = ""
}

func (m Model) IsVisible() bool { return m.visible }

func (m Model) Cwd() string { return m.cwd }

func (m Model) Query() string { return m.query }

func (m Model) FilteredCount() int { return len(m.filtered) }

func (m Model) SelectedPath() string {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return ""
	}
	return m.entries[m.filtered[m.selected]].Path
}

func (m *Model) HandleKey(keyStr string) *Result {
	if !m.visible {
		return nil
	}
	switch keyStr {
	case "enter":
		return m.selectCurrent()
	case "esc":
		m.Close()
		return nil
	case "down", "ctrl+n", "j":
		if m.selected < len(m.filtered)-1 {
			m.selected++
		}
		return nil
	case "up", "ctrl+p", "k":
		if m.selected > 0 {
			m.selected--
		}
		return nil
	case "backspace":
		if m.query != "" {
			m.query = m.query[:len(m.query)-1]
			m.selected = 0
			m.filter()
		} else {
			m.goParent()
		}
		return nil
	case "h":
		if m.query == "" {
			m.goParent()
		} else {
			m.query += keyStr
			m.selected = 0
			m.filter()
		}
		return nil
	}

	if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] <= 126 {
		m.query += keyStr
		m.selected = 0
		m.filter()
	}
	return nil
}

func (m *Model) selectCurrent() *Result {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return nil
	}
	item := m.entries[m.filtered[m.selected]]
	if item.Parent || item.Dir {
		m.setDir(item.Path)
		m.query = ""
		m.selected = 0
		m.filter()
		return nil
	}
	path := item.Path
	m.Close()
	return &Result{Path: path}
}

func (m *Model) goParent() {
	if m.cwd == "" {
		return
	}
	parent := filepath.Dir(m.cwd)
	if parent == m.cwd {
		return
	}
	m.setDir(parent)
}

func (m *Model) setDir(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	infos, err := os.ReadDir(abs)
	if err != nil {
		m.err = err.Error()
		m.entries = nil
		m.filtered = nil
		m.cwd = abs
		return
	}
	m.err = ""
	m.cwd = abs
	entries := []entry{}
	parent := filepath.Dir(abs)
	if parent != abs {
		entries = append(entries, entry{Name: "..", Path: parent, Dir: true, Parent: true})
	}
	for _, info := range infos {
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(abs, name)
		item := entry{Name: name, Path: full, Dir: info.IsDir()}
		if !info.IsDir() {
			if stat, err := info.Info(); err == nil {
				item.Size = stat.Size()
			}
		}
		entries = append(entries, item)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Parent != b.Parent {
			return a.Parent
		}
		if a.Dir != b.Dir {
			return a.Dir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	m.entries = entries
	m.selected = 0
	m.filter()
}

func (m *Model) filter() {
	m.filtered = nil
	q := text.Fold(m.query)
	for i, item := range m.entries {
		if q == "" || strings.Contains(text.Fold(item.Name), q) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = len(m.filtered) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m Model) View(termWidth int) string { return m.renderBox(termWidth) }

func (m Model) ViewOverlay(termWidth, termHeight int, background string) string {
	if !m.visible {
		return background
	}
	box := m.renderBox(termWidth)
	if box == "" {
		return background
	}
	result := overlay.DimmedOverlay(termWidth, termHeight, background, box, 0.5)
	lines := strings.Split(result, "\n")
	if len(lines) > termHeight {
		lines = lines[:termHeight]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBox(termWidth int) string {
	if !m.visible {
		return ""
	}
	overlayWidth := termWidth * 55 / 100
	if overlayWidth < 50 {
		overlayWidth = 50
	}
	if overlayWidth > 100 {
		overlayWidth = 100
	}
	innerWidth := overlayWidth - 4
	bg := styles.Background

	title := lipgloss.NewStyle().
		Bold(true).
		Background(bg).
		Foreground(styles.Primary).
		Render("Attach File")

	cwd := truncate.StringWithTail(m.cwd, uint(innerWidth), "…")
	cwdLine := lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted).Render(cwd)

	inputText := m.query + "█"
	if m.query == "" {
		placeholder := lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted).Render("Type to filter files...")
		inputText = "█ " + placeholder
	}
	input := lipgloss.NewStyle().
		BorderStyle(lipgloss.Border{Left: "▌"}).
		BorderLeft(true).
		BorderForeground(styles.Primary).
		BorderBackground(bg).
		PaddingLeft(1).
		Background(bg).
		Foreground(styles.TextPrimary).
		Render(inputText)

	rows := m.renderRows(innerWidth)
	footer := lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted).
		Render("[enter] open/attach   [h/backspace] parent   [esc] cancel")
	content := title + "\n" + cwdLine + "\n" + input + "\n\n" + strings.Join(rows, "\n") + "\n\n" + footer
	content = messages.ReapplyBgAfterResets(content, messages.BgANSI()+messages.FgANSI())

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		BorderBackground(bg).
		Background(bg).
		Padding(1, 1).
		Width(overlayWidth).
		Render(content)
}

func (m Model) renderRows(innerWidth int) []string {
	bg := styles.Background
	if m.err != "" {
		return []string{lipgloss.NewStyle().Background(styles.Background).Foreground(styles.Error).Render("Error: " + m.err)}
	}
	if len(m.filtered) == 0 {
		return []string{lipgloss.NewStyle().Background(styles.Background).Foreground(styles.TextMuted).Italic(true).Render("No matching files")}
	}
	maxVisible := 12
	total := len(m.filtered)
	if maxVisible > total {
		maxVisible = total
	}
	start := 0
	if m.selected >= maxVisible {
		start = m.selected - maxVisible + 1
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	showScrollbar := total > maxVisible
	contentWidth := innerWidth - 1
	if showScrollbar {
		contentWidth--
	}

	thumbStart, thumbEnd := 0, 0
	if showScrollbar {
		thumbHeight := maxVisible * maxVisible / total
		if thumbHeight < 1 {
			thumbHeight = 1
		}
		denom := total - maxVisible
		if denom < 1 {
			denom = 1
		}
		thumbStart = start * (maxVisible - thumbHeight) / denom
		thumbEnd = thumbStart + thumbHeight
	}

	thumbStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.Primary)
	trackStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.Border)
	var rows []string
	for i := start; i < end; i++ {
		item := m.entries[m.filtered[i]]
		selected := i == m.selected
		line := formatEntry(item)
		if lipgloss.Width(line) > contentWidth {
			line = truncate.StringWithTail(line, uint(contentWidth), "…")
		}
		if pad := contentWidth - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		nameStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.TextPrimary)
		if item.Dir {
			nameStyle = nameStyle.Foreground(styles.Primary)
		}
		if selected {
			nameStyle = nameStyle.Bold(true).Foreground(styles.Accent)
		}
		line = nameStyle.Render(line)
		indicator := " "
		if selected {
			indicator = lipgloss.NewStyle().Background(bg).Foreground(styles.Accent).Render("▌")
		}
		row := indicator + line
		if showScrollbar {
			rel := i - start
			if rel >= thumbStart && rel < thumbEnd {
				row += thumbStyle.Render("█")
			} else {
				row += trackStyle.Render("│")
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func formatEntry(item entry) string {
	if item.Parent {
		return "↩  .."
	}
	if item.Dir {
		return "📁 " + item.Name + "/"
	}
	return fmt.Sprintf("📄 %s  %s", item.Name, formatSize(item.Size))
}

func formatSize(size int64) string {
	const kb = 1024
	const mb = 1024 * kb
	switch {
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%d KB", size/kb)
	default:
		return "<1 KB"
	}
}
