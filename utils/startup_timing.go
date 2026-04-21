package utils

import (
	"log"
	"time"

	"go.uber.org/zap"
)

// StartupTimer tracks elapsed startup time between key phases.
type StartupTimer struct {
	name       string
	logger     *zap.SugaredLogger
	startedAt  time.Time
	lastPhase  time.Time
}

// NewStartupTimer creates a phase timer for startup instrumentation.
func NewStartupTimer(name string, logger *zap.SugaredLogger) *StartupTimer {
	now := time.Now()
	return &StartupTimer{
		name:      name,
		logger:    logger,
		startedAt: now,
		lastPhase: now,
	}
}

// Mark records timing for a startup phase.
func (t *StartupTimer) Mark(phase string) {
	now := time.Now()
	phaseMs := now.Sub(t.lastPhase).Milliseconds()
	totalMs := now.Sub(t.startedAt).Milliseconds()
	t.lastPhase = now

	if t.logger != nil {
		t.logger.Infow("startup_phase",
			"name", t.name,
			"phase", phase,
			"phase_ms", phaseMs,
			"total_ms", totalMs,
		)
		return
	}

	log.Printf("startup_phase name=%s phase=%s phase_ms=%d total_ms=%d", t.name, phase, phaseMs, totalMs)
}

// LogAsync logs duration for asynchronous startup tasks.
func LogAsyncStartupPhase(logger *zap.SugaredLogger, name, phase string, startedAt time.Time, err error) {
	durMs := time.Since(startedAt).Milliseconds()
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}

	if logger != nil {
		logger.Infow("startup_phase_async",
			"name", name,
			"phase", phase,
			"duration_ms", durMs,
			"status", status,
			"error", errMsg,
		)
		return
	}

	log.Printf("startup_phase_async name=%s phase=%s duration_ms=%d status=%s error=%s", name, phase, durMs, status, errMsg)
}
