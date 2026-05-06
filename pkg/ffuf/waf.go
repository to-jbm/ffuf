package ffuf

import (
	"fmt"
	"time"
)

// evalWAF returns true if the response matches any of the configured WAF/rate-limit
// detector rules (-wms / -wmw / -wml / -wmr). Semantics are OR across rules: any
// rule matching is enough to flag the response as a WAF/rate-limit response.
func (j *Job) evalWAF(resp Response) bool {
	if j.Config == nil || len(j.Config.WAFMatchers) == 0 {
		return false
	}
	for _, m := range j.Config.WAFMatchers {
		match, err := m.Filter(&resp)
		if err != nil {
			continue
		}
		if match {
			return true
		}
	}
	return false
}

// recordWAFResult feeds a single response classification into the consecutive
// counters. When the WAF threshold is reached, it triggers the adaptive
// backoff sleep (synchronously from the caller goroutine).
//
// While a backoff is in progress this function is a no-op (any in-flight
// goroutines completing during the pause are ignored on purpose, otherwise
// the next escalation would fire immediately when the pause ends).
func (j *Job) recordWAFResult(isWAF bool) {
	if j.Config == nil || len(j.Config.WAFMatchers) == 0 || len(j.Config.WAFTimes) == 0 {
		return
	}
	threshold := j.Config.WAFThreshold
	if threshold <= 0 {
		threshold = 10
	}

	j.wafMu.Lock()
	if j.wafBackingOff {
		j.wafMu.Unlock()
		return
	}

	triggered := false
	var duration time.Duration

	if isWAF {
		j.wafConsec++
		j.nonWafConsec = 0
		if j.wafConsec >= threshold {
			j.wafBackingOff = true
			j.wafCancelCh = make(chan struct{})
			j.wafPauseWg.Add(1)
			duration = j.nextBackoffDurationLocked()
			j.wafConsec = 0
			triggered = true
			// Set the public flag so the output layer starts buffering
			// stdout prints right away (before we even enter the sleep).
			j.Config.SetWAFBackingOff(true)
		}
	} else {
		// Only count toward "recovery" if we previously saw any WAF activity
		// (i.e. the ladder is not already at index 0 with no current streak).
		if j.wafConsec > 0 || j.wafEscalIdx > 0 {
			j.nonWafConsec++
			if j.nonWafConsec >= threshold {
				j.nonWafConsec = 0
				j.wafConsec = 0
				j.wafEscalIdx = 0
			}
		}
	}
	j.wafMu.Unlock()

	if triggered {
		j.doWAFPause(duration)
	}
}

// nextBackoffDurationLocked returns the next pause duration based on the
// escalation ladder and advances the index (capped at the last value).
// Caller MUST hold j.wafMu.
func (j *Job) nextBackoffDurationLocked() time.Duration {
	times := j.Config.WAFTimes
	idx := j.wafEscalIdx
	if idx >= len(times) {
		idx = len(times) - 1
	}
	d := time.Duration(times[idx]) * time.Second
	if j.wafEscalIdx < len(times)-1 {
		j.wafEscalIdx++
	}
	return d
}

// doWAFPause performs the actual cancellable sleep and releases the worker
// barrier when finished. The sleep can be interrupted by:
//   - job context cancellation (Ctrl-C / job stop)
//   - an explicit CancelWAFBackoff() call (e.g. user entering interactive mode)
func (j *Job) doWAFPause(d time.Duration) {
	if j.Output != nil {
		j.Output.Warning(fmt.Sprintf("WAF/rate-limit threshold reached: pausing for %s (Ctrl-C to abort, ENTER to skip and enter interactive mode)", d))
	}

	j.wafMu.Lock()
	cancelCh := j.wafCancelCh
	j.wafMu.Unlock()

	cancelled := false
	select {
	case <-j.Config.Context.Done():
		cancelled = true
	case <-cancelCh:
		cancelled = true
	case <-time.After(d):
	}

	j.wafMu.Lock()
	j.wafBackingOff = false
	j.wafConsec = 0
	j.nonWafConsec = 0
	j.wafCancelCh = nil
	j.wafMu.Unlock()
	// Clear the public flag BEFORE flushing so any results that race in at
	// this point go to stdout directly instead of bouncing through the
	// pending buffer.
	j.Config.SetWAFBackingOff(false)
	j.wafPauseWg.Done()

	if j.Output != nil && j.Running {
		if cancelled {
			j.Output.Info("WAF/rate-limit backoff cancelled")
		} else {
			j.Output.Info("WAF/rate-limit backoff complete, resuming")
		}
		// Print any results captured during the pause so the user can
		// still see the matches that completed in flight.
		j.Output.FlushPendingResults()
	}
}

// CancelWAFBackoff cancels any in-progress WAF/rate-limit backoff sleep and
// resets the escalation ladder back to its first step. Safe to call at any
// time (no-op if no backoff is in progress).
//
// The detector remains armed afterwards; only the current pause and the
// escalation history are cleared. Pending stdout prints buffered during the
// pause are flushed so the user is not surprised by missing output.
func (j *Job) CancelWAFBackoff() {
	j.wafMu.Lock()
	j.wafConsec = 0
	j.nonWafConsec = 0
	j.wafEscalIdx = 0
	wasPaused := j.wafBackingOff
	if j.wafBackingOff && j.wafCancelCh != nil {
		select {
		case <-j.wafCancelCh:
			// already closed
		default:
			close(j.wafCancelCh)
		}
	}
	j.wafMu.Unlock()
	if wasPaused {
		// Make sure the output flag is cleared even if doWAFPause has
		// not yet observed the cancel (e.g. when called from the Ctrl-C
		// monitor before the select returns).
		j.Config.SetWAFBackingOff(false)
		if j.Output != nil {
			j.Output.FlushPendingResults()
		}
	}
}
