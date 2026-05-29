package stat

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
)

// Semantic palette. Every color has a single, named role — never reach for a
// numeric index when adding a new one.
var (
	cTitle  = lipgloss.Color("#7DD3FC") // cyan-300
	cAccent = lipgloss.Color("#F472B6") // pink-400
	cOK     = lipgloss.Color("#34D399") // emerald-400
	cWarn   = lipgloss.Color("#FBBF24") // amber-400
	cErr    = lipgloss.Color("#F87171") // red-400
	cInfo   = lipgloss.Color("#A78BFA") // violet-400
	cMuted  = lipgloss.Color("#94A3B8") // slate-400
	cBorder = lipgloss.Color("#64748B") // slate-500 — also used as the bar track

	// Storage bar segments.
	cRecent = lipgloss.Color("#22D3EE") // cyan-400
	cStale  = lipgloss.Color("#F59E0B") // amber-500
	cKeep   = lipgloss.Color("#A78BFA") // violet-400
)

// renderSnapshot is the single entry point for the dashboard. It writes to
// stderr and silently degrades to plain text when colors are unavailable.
func renderSnapshot(s *repoSnapshot) {
	if !colorEnabled() {
		fmt.Fprint(os.Stderr, plainReport(s))
		return
	}
	width := cardWidth()

	var out strings.Builder
	out.WriteString(headerCard(s, width))
	out.WriteString("\n")
	out.WriteString(healthCard(s, width))
	out.WriteString("\n")
	out.WriteString(sectionCard("Identity", s.Identity, width))
	out.WriteString("\n")
	out.WriteString(sectionCard("Remote", s.Remote, width))
	out.WriteString("\n")
	out.WriteString(sectionCard("Repository", s.Repository, width))
	out.WriteString("\n")
	out.WriteString(sectionCard("References", s.References, width))
	out.WriteString("\n")
	out.WriteString(storageCard(s, width))
	if s.LFS != nil {
		out.WriteString("\n")
		out.WriteString(lfsCard(s, width))
	}
	fmt.Fprintln(os.Stderr, out.String())
}

// colorEnabled reports whether stderr can host the rich output.
func colorEnabled() bool {
	if !term.IsTerminal(os.Stderr.Fd()) {
		return false
	}
	return term.StderrLevel.SupportColor()
}

// cardWidth returns the outer width every card should be rendered at.
func cardWidth() int {
	w, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || w <= 0 {
		return 84
	}
	switch {
	case w > 100:
		return 100
	case w < 60:
		return 60
	default:
		return w - 2
	}
}

// --- Cards ----------------------------------------------------------------

func headerCard(s *repoSnapshot, width int) string {
	name := filepath.Base(filepath.Clean(s.RepoPath))
	if name == "." || name == "/" || name == "" {
		name = s.RepoPath
	}

	logo := lipgloss.NewStyle().Foreground(cAccent).Bold(true).Render(" HOT STAT ")
	title := lipgloss.NewStyle().Foreground(cTitle).Bold(true).Render(name)
	sub := lipgloss.NewStyle().Foreground(cMuted).Render(s.RepoPath)
	if s.GitVersion != "" {
		sub += lipgloss.NewStyle().Foreground(cMuted).Render("  •  git " + s.GitVersion)
	}

	return cardBox(cAccent, width).Render(logo + "  " + title + "\n" + sub)
}

func healthCard(s *repoSnapshot, width int) string {
	score, grade, gradeColor := healthScore(s)
	scoreText := lipgloss.NewStyle().
		Foreground(gradeColor).
		Bold(true).
		Render(fmt.Sprintf("%d/100  %s", score, grade))

	var body strings.Builder
	body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(cTitle).Render("Health  "))
	body.WriteString(scoreText)
	if tags := healthTags(s); len(tags) > 0 {
		body.WriteString("\n")
		body.WriteString(strings.Join(tags, "  "))
	}
	body.WriteString("\n")
	body.WriteString(healthBar(score, width-4))

	return cardBox(cBorder, width).Render(body.String())
}

func sectionCard(title string, items []checkItem, width int) string {
	if len(items) == 0 {
		return ""
	}
	innerWidth := width - 4 // border + padding

	labelWidth := 0
	for _, it := range items {
		if l := lipgloss.Width(it.Label); l > labelWidth {
			labelWidth = l
		}
	}
	if labelWidth > 24 {
		labelWidth = 24
	}

	lines := make([]string, 0, len(items)+1)
	lines = append(lines, lipgloss.NewStyle().Foreground(cTitle).Bold(true).Render(title))
	for _, it := range items {
		icon, iconColor := stateBadge(it.State)
		label := lipgloss.NewStyle().Foreground(cMuted).Width(labelWidth).Render(it.Label)
		row := lipgloss.NewStyle().Foreground(iconColor).Render(icon) + " " + label + "  " + valueStyle(it)
		if it.Hint != "" {
			row += lipgloss.NewStyle().Foreground(cMuted).Italic(true).Render("  ⤷ " + it.Hint)
		}
		lines = append(lines, clipToWidth(row, innerWidth))
	}
	return cardBox(cBorder, width).Render(strings.Join(lines, "\n"))
}

func storageCard(s *repoSnapshot, width int) string {
	st := s.Storage
	innerWidth := max(width-4, 30)

	// Total disk usage is the scale for every progress row below — even an
	// empty repo has a few bytes on disk, so divide-by-zero is guarded later.
	total := uint64(st.DiskSize)
	if total == 0 {
		total = st.RecentSize + st.StaleSize + st.KeepSize
	}

	rows := []struct {
		label string
		size  uint64
		color color.Color
		hint  string
	}{
		{"recent", st.RecentSize, cRecent, "reachable & fresh objects"},
		{"stale", st.StaleSize, cStale, "unreachable / gc candidates"},
		{"keep", st.KeepSize, cKeep, "pinned packfiles (.keep)"},
	}

	// Reserve room: "label " (8) + " size " (~11) + " pct " (~7) = 26
	const sideCols = 26
	barWidth := max(innerWidth-sideCols, 12)

	var body strings.Builder
	body.WriteString(lipgloss.NewStyle().Foreground(cTitle).Bold(true).Render("Storage"))
	body.WriteString("\n")

	// Headline: a single, unambiguous number.
	body.WriteString(inlineKV("disk usage", strengthen.FormatSize(st.DiskSize)))
	body.WriteString("\n")

	if total == 0 {
		body.WriteString(lipgloss.NewStyle().Foreground(cMuted).Render("(no packed objects yet)"))
	} else {
		for _, r := range rows {
			body.WriteString("\n")
			body.WriteString(progressRow(r.label, r.size, total, barWidth, r.color, r.hint))
		}
		body.WriteString("\n\n")
		body.WriteString(strings.Join([]string{
			inlineKV("objects", strengthen.FormatSizeU(st.LooseSize+st.PackSize)),
			inlineKV("loose", strconv.FormatUint(st.LooseCount, 10)),
			inlineKV("packs", strconv.FormatUint(st.PackCount, 10)),
		}, "  •  "))
	}

	if tips := storageTips(st); tips != "" {
		body.WriteString("\n")
		body.WriteString(tips)
	}
	return cardBox(cBorder, width).Render(body.String())
}

// progressRow draws one labeled line with its own scale, e.g.
//
//	recent  ████████████████████░░░░░  5.99 MiB  72.4%
//
// Empty values are still shown (as an empty track) so the user can compare
// rows visually.
func progressRow(label string, value, total uint64, barWidth int, c color.Color, hint string) string {
	pct := 0.0
	if total > 0 {
		pct = float64(value) * 100 / float64(total)
	}
	filled := 0
	if total > 0 {
		filled = int(uint64(barWidth) * value / total)
	}
	if filled > barWidth {
		filled = barWidth
	}

	// Avoid emitting empty ANSI-wrapped strings — they leave a stray reset on
	// some terminals.
	var bar string
	if filled > 0 {
		bar = lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled))
	}
	if filled < barWidth {
		bar += lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("░", barWidth-filled))
	}

	labelCol := lipgloss.NewStyle().
		Foreground(cMuted).
		Width(7).
		Render(label)
	sizeCol := lipgloss.NewStyle().
		Foreground(cTitle).
		Bold(true).
		Width(10).
		Align(lipgloss.Right).
		Render(strengthen.FormatSizeU(value))
	pctCol := lipgloss.NewStyle().
		Foreground(cMuted).
		Width(6).
		Align(lipgloss.Right).
		Render(fmt.Sprintf("%.1f%%", pct))

	row := labelCol + " " + bar + " " + sizeCol + " " + pctCol
	// When the slice is dominant (>=40%) we add a description on its own line
	// so the user can learn what each segment means without crowding the bar.
	if hint != "" && pct >= 40 {
		row += "\n" + lipgloss.NewStyle().
			Foreground(cMuted).
			Italic(true).
			Render("        ⤷ "+hint)
	}
	return row
}

func lfsCard(s *repoSnapshot, width int) string {
	head := lipgloss.NewStyle().Foreground(cTitle).Bold(true).Render("LFS")
	line := fmt.Sprintf("%s   %s",
		inlineKV("count", strconv.FormatUint(s.LFS.Count, 10)),
		inlineKV("size", strengthen.FormatSizeU(s.LFS.Size)),
	)
	return cardBox(cBorder, width).Render(head + "\n" + line)
}

// cardBox is the shared rounded-border container all cards live inside.
func cardBox(border color.Color, width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(width)
}

// --- Health ---------------------------------------------------------------

// healthScore returns the score in [0, 100], a textual grade and a color that
// tracks the grade. Tweak the deductions, not the grading bands.
func healthScore(s *repoSnapshot) (int, string, color.Color) {
	score := 100
	if s.UnsafeURL {
		score -= 25
	}
	if s.HasHooks {
		score -= 5
	}
	for _, it := range s.Identity {
		if it.State == stateError {
			score -= 15
			break
		}
	}
	// Stale objects dominating the storage is a strong gc signal.
	if total := s.Storage.RecentSize + s.Storage.StaleSize + s.Storage.KeepSize; total > 0 {
		stalePct := float64(s.Storage.StaleSize) / float64(total)
		if stalePct > 0.30 {
			score -= 10
		}
		if stalePct > 0.50 {
			score -= 10
		}
	}
	if s.Storage.PackCount > 20 {
		score -= 5
	}
	if s.Storage.LooseCount > 5000 {
		score -= 5
	}
	if s.OversizedCount > 0 {
		score -= 10
	}
	if score < 0 {
		score = 0
	}
	switch {
	case score >= 90:
		return score, "EXCELLENT", cOK
	case score >= 75:
		return score, "GOOD", cOK
	case score >= 60:
		return score, "FAIR", cWarn
	case score >= 40:
		return score, "POOR", cWarn
	default:
		return score, "CRITICAL", cErr
	}
}

func healthTags(s *repoSnapshot) []string {
	dot := func(text string, c color.Color) string {
		return lipgloss.NewStyle().Foreground(c).Render("● " + text)
	}
	var tags []string
	if s.UnsafeURL {
		tags = append(tags, dot("unsafe remote", cErr))
	} else {
		tags = append(tags, dot("remote", cOK))
	}
	if s.HasHooks {
		tags = append(tags, dot("custom hooks", cWarn))
	}
	if s.Sparse {
		tags = append(tags, dot("sparse", cInfo))
	}
	if s.Partial {
		tags = append(tags, dot("partial", cInfo))
	}
	if s.OversizedCount > 0 {
		tags = append(tags, dot(strconv.Itoa(s.OversizedCount)+" oversize", cWarn))
	}
	return tags
}

func healthBar(score, width int) string {
	if width < 10 {
		width = 10
	}
	filled := score * width / 100
	switch {
	case filled < 0:
		filled = 0
	case filled > width:
		filled = width
	}
	barColor := cErr
	switch {
	case score >= 75:
		barColor = cOK
	case score >= 60:
		barColor = cWarn
	}
	bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled))
	track := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("░", width-filled))
	return bar + track
}

// --- Storage helpers ------------------------------------------------------

func storageTips(st storageSummary) string {
	var tips []string
	if st.LooseCount > 5000 {
		tips = append(tips, fmt.Sprintf("loose objects = %d, run `git gc` to pack them", st.LooseCount))
	}
	if st.PackCount > 20 {
		tips = append(tips, fmt.Sprintf("%d packfiles, consider `git repack -ad`", st.PackCount))
	}
	if len(tips) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tips))
	for _, t := range tips {
		lines = append(lines,
			lipgloss.NewStyle().Foreground(cWarn).Render("→ ")+
				lipgloss.NewStyle().Foreground(cMuted).Render(t))
	}
	return strings.Join(lines, "\n")
}

// --- Cell helpers ---------------------------------------------------------

func stateBadge(s checkState) (string, color.Color) {
	switch s {
	case stateOK:
		return "✓", cOK
	case stateWarn:
		return "!", cWarn
	case stateError:
		return "✗", cErr
	default:
		return "•", cInfo
	}
}

func valueStyle(it checkItem) string {
	c := cTitle
	switch it.State {
	case stateError:
		c = cErr
	case stateWarn:
		c = cWarn
	case stateInfo:
		c = cInfo
	}
	return lipgloss.NewStyle().Foreground(c).Render(it.Value)
}

// inlineKV renders a compact "key: value" pair for one-line layouts.
func inlineKV(key, value string) string {
	return lipgloss.NewStyle().Foreground(cMuted).Render(key+":") + " " +
		lipgloss.NewStyle().Foreground(cTitle).Bold(true).Render(value)
}

// clipToWidth truncates an ANSI-styled line to max display columns while
// keeping the trailing reset intact.
func clipToWidth(s string, maxVal int) string {
	if maxVal <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxVal {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(maxVal).Render(s)
}

// --- Plain fallback -------------------------------------------------------

func plainReport(s *repoSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Location: %s\n", s.RepoPath)
	if s.GitVersion != "" {
		fmt.Fprintf(&b, "Git Version: %s\n", s.GitVersion)
	}
	for _, sec := range []struct {
		name  string
		items []checkItem
	}{
		{"Identity", s.Identity},
		{"Remote", s.Remote},
		{"Repository", s.Repository},
		{"References", s.References},
	} {
		for _, it := range sec.items {
			tag := "*"
			switch it.State {
			case stateOK:
				tag = "ok"
			case stateWarn:
				tag = "warn"
			case stateError:
				tag = "err"
			}
			fmt.Fprintf(&b, "[%s] %s: %s", tag, it.Label, it.Value)
			if it.Hint != "" {
				fmt.Fprintf(&b, "  (%s)", it.Hint)
			}
			b.WriteByte('\n')
		}
	}
	st := s.Storage
	fmt.Fprintf(&b, "Storage: disk=%s objects=%s loose=%d packs=%d recent=%s stale=%s keep=%s\n",
		strengthen.FormatSize(st.DiskSize),
		strengthen.FormatSizeU(st.LooseSize+st.PackSize),
		st.LooseCount,
		st.PackCount,
		strengthen.FormatSizeU(st.RecentSize),
		strengthen.FormatSizeU(st.StaleSize),
		strengthen.FormatSizeU(st.KeepSize),
	)
	if s.LFS != nil {
		fmt.Fprintf(&b, "LFS: count=%d size=%s\n", s.LFS.Count, strengthen.FormatSizeU(s.LFS.Size))
	}
	return b.String()
}
