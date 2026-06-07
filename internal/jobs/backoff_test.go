package jobs

import (
	"testing"
	"time"
)

func TestRetryBackoff(t *testing.T) {
	sched := []time.Duration{time.Minute, 5 * time.Minute}
	if RetryBackoff(0, sched) != time.Minute {
		t.Fatal("attempt 0")
	}
	if RetryBackoff(9, sched) != 5*time.Minute {
		t.Fatal("cap")
	}
}
