package crawler

import (
	"context"
	"sync"
)

// JobManager manages cancel functions for running crawler jobs.
// It allows external cancellation of jobs by their run ID.
type JobManager struct {
	mu      sync.RWMutex
	cancels map[string]context.CancelFunc
}

// NewJobManager creates a new JobManager instance.
func NewJobManager() *JobManager {
	return &JobManager{
		cancels: make(map[string]context.CancelFunc),
	}
}

// Register stores a cancel function for a job.
// This should be called when a new job starts.
func (jm *JobManager) Register(runID string, cancel context.CancelFunc) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.cancels[runID] = cancel
}

// Cancel invokes the cancel function for a job if it exists.
// Returns true if the job was found and cancelled.
func (jm *JobManager) Cancel(runID string) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, ok := jm.cancels[runID]; ok {
		cancel()
		delete(jm.cancels, runID)
		return true
	}
	return false
}

// Unregister removes a job's cancel function.
// This should be called when a job completes (success, fail, or timeout).
func (jm *JobManager) Unregister(runID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.cancels, runID)
}

// IsRunning checks if a job is currently registered (running).
func (jm *JobManager) IsRunning(runID string) bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	_, ok := jm.cancels[runID]
	return ok
}
