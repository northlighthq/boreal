package onboarding

import "time"

const contentFade = 380 * time.Millisecond

func ease(t float64) float64 {
	return t * t * (3 - 2*t)
}

func clamp01(t float64) float64 {
	return min(max(t, 0), 1)
}
