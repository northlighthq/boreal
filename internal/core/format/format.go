package format

import (
	"fmt"
	"time"
)

func Size(n int64) string {
	const unit = 1024

	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := int64(unit), 0

	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func FileTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	now := time.Now()

	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return "Today, " + t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2, 15:04")
	default:
		return t.Format("Jan 2, 2006")
	}
}

func Uptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
