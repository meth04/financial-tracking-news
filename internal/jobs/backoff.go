package jobs

import "time"

func RetryBackoff(attempts int, schedule []time.Duration) time.Duration {
	if len(schedule) == 0 {
		schedule = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	}
	if attempts < 0 {
		attempts = 0
	}
	if attempts >= len(schedule) {
		return schedule[len(schedule)-1]
	}
	return schedule[attempts]
}
