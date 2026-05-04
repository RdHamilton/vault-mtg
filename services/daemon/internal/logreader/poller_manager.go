package logreader

import (
	"context"
	"fmt"
	"sync"
)

// PollerManager manages multiple log file pollers simultaneously.
type PollerManager struct {
	pollers   map[string]*Poller
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	updates   chan *LogEntry
	errChan   chan error
	running   bool
	runningMu sync.RWMutex
	wg        sync.WaitGroup
}

// PollerManagerConfig holds configuration for a PollerManager.
type PollerManagerConfig struct {
	BufferSize int
}

// DefaultPollerManagerConfig returns a PollerManagerConfig with sensible defaults.
func DefaultPollerManagerConfig() *PollerManagerConfig {
	return &PollerManagerConfig{
		BufferSize: 1000,
	}
}

// NewPollerManager creates a new PollerManager.
func NewPollerManager(config *PollerManagerConfig) *PollerManager {
	if config == nil {
		config = DefaultPollerManagerConfig()
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &PollerManager{
		pollers: make(map[string]*Poller),
		ctx:     ctx,
		cancel:  cancel,
		updates: make(chan *LogEntry, config.BufferSize),
		errChan: make(chan error, 100),
	}
}

// AddPoller adds a new poller to the manager.
func (pm *PollerManager) AddPoller(key string, config *PollerConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, exists := pm.pollers[key]; exists {
		return fmt.Errorf("poller with key %q already exists", key)
	}
	poller, err := NewPoller(config)
	if err != nil {
		return fmt.Errorf("create poller: %w", err)
	}
	pm.pollers[key] = poller
	if pm.isRunningLocked() {
		pm.startPoller(key, poller)
	}
	return nil
}

// RemovePoller removes and stops a poller from the manager.
func (pm *PollerManager) RemovePoller(key string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	poller, exists := pm.pollers[key]
	if !exists {
		return fmt.Errorf("poller with key %q not found", key)
	}
	poller.Stop()
	delete(pm.pollers, key)
	return nil
}

// Start starts all pollers in the manager.
func (pm *PollerManager) Start() <-chan *LogEntry {
	pm.runningMu.Lock()
	if pm.running {
		pm.runningMu.Unlock()
		return pm.updates
	}
	pm.running = true
	pm.runningMu.Unlock()
	pm.mu.RLock()
	for key, poller := range pm.pollers {
		pm.startPoller(key, poller)
	}
	pm.mu.RUnlock()
	return pm.updates
}

func (pm *PollerManager) startPoller(key string, poller *Poller) {
	updates := poller.Start()
	errChan := poller.Errors()
	pollerKey := key
	pm.wg.Go(func() {
		for {
			select {
			case <-pm.ctx.Done():
				return
			case entry, ok := <-updates:
				if !ok {
					return
				}
				select {
				case pm.updates <- entry:
				case <-pm.ctx.Done():
					return
				}
			case err, ok := <-errChan:
				if !ok {
					return
				}
				select {
				case pm.errChan <- fmt.Errorf("poller %q: %w", pollerKey, err):
				default:
				}
			}
		}
	})
}

// Stop stops all pollers in the manager.
func (pm *PollerManager) Stop() {
	pm.runningMu.Lock()
	if !pm.running {
		pm.runningMu.Unlock()
		return
	}
	pm.running = false
	pm.runningMu.Unlock()
	pm.mu.Lock()
	for _, poller := range pm.pollers {
		poller.Stop()
	}
	pm.mu.Unlock()
	pm.cancel()
	pm.wg.Wait()
	close(pm.updates)
}

// Errors returns a channel that receives errors from all pollers.
func (pm *PollerManager) Errors() <-chan error {
	return pm.errChan
}

// IsRunning returns whether the manager is currently running.
func (pm *PollerManager) IsRunning() bool {
	pm.runningMu.RLock()
	defer pm.runningMu.RUnlock()
	return pm.running
}

func (pm *PollerManager) isRunningLocked() bool {
	pm.runningMu.RLock()
	defer pm.runningMu.RUnlock()
	return pm.running
}

// PollerCount returns the number of pollers being managed.
func (pm *PollerManager) PollerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.pollers)
}
