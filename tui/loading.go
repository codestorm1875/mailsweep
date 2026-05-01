package tui

import "fmt"

func renderLoading(fetched, total int) string {
	title := titleStyle.Render("mailsweep")

	if total == 0 {
		spinner := statusStyle.Render("⏳ Scanning your mailbox...")
		return fmt.Sprintf("\n%s\n\n%s\n", title, spinner)
	}

	pct := float64(fetched) / float64(total) * 100
	progress := statusStyle.Render(
		fmt.Sprintf("⏳ Fetching email metadata... %d / %d (%.0f%%)", fetched, total, pct))

	barWidth := 40
	filled := int(float64(fetched) / float64(total) * float64(barWidth))
	bar := fmt.Sprintf("[%s%s]",
		repeat("█", filled),
		repeat("░", barWidth-filled))

	coloredBar := barLowStyle.Render(bar)

	return fmt.Sprintf("\n%s\n\n%s\n%s\n", title, progress, coloredBar)
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
