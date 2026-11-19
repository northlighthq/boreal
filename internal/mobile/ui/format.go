package ui

import (
	"strconv"
	"time"
)

const (
	day   = 24 * time.Hour
	week  = 7 * day
	month = 30 * day
	year  = 365 * day
)

func Ago(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := now.Sub(t)

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return strconv.Itoa(int(d/time.Minute)) + "m"
	case d < day:
		return strconv.Itoa(int(d/time.Hour)) + "h"
	case d < week:
		return strconv.Itoa(int(d/day)) + "d"
	case d < month:
		return strconv.Itoa(int(d/week)) + "w"
	case d < year:
		return strconv.Itoa(int(d/month)) + "mo"
	}

	return strconv.Itoa(int(d/year)) + "y"
}
