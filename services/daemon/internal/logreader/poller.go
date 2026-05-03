package logreader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Poller monitors a log file for new entries and sends them through a channel.
// It tracks file position to only read new entries and handles log file rotation.
type Poller struct {
	path          string
	interval      time.Duration
	useFileEvents bool
	watcher       *fsnotify.Watcher
	lastPos       int64
	lastSize      int64
	lastMod       time.Time
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	updates       chan *LogEntry
	errChan       chan error
	done          chan struct{}
	running       bool
	runningMu     sync.RWMutex
	metrics       *PollerMetrics
	enableMetrics bool
}

// PollerMetrics tracks performance metrics for the poller.
type PollerMetrics struct {
	mu                    sync.RWMutex
	PollCount             uint64
	EntriesProcessed      uint64
	ErrorCount            uint64
	TotalProcessingTime   time.Duration
	LastPollTime          time.Time
	LastPollDuration      time.Duration
	AverageEntriesPerPoll float64
}

// PollerConfig holds configuration for a Poller.
type PollerConfig struct {
	// Path is the path to the log file to monitor.
	Path string

	// Interval is how often to check for new entries when using polling.
	// Default: 2 seconds
	Interval time.Duration

	// BufferSize is the size of the updates channel buffer.
	// Default: 100
	BufferSize int

	// UseFileEvents enables file system event monitoring (fsnotify).
	// Default: true
	UseFileEvents bool

	// EnableMetrics enables collection of performance metrics.
	// Default: false
	EnableMetrics bool

	// ReadFromStart if true, reads the entire log file from the beginning
	// on first start. If false, only reads new entries added after start.
	ReadFromStart bool
}

// DefaultPollerConfig returns a PollerConfig with sensible defaults.
func DefaultPollerConfig(path string) *PollerConfig {
	return &PollerConfig{
		Path:          path,
		Interval:      2 * time.Second,
		BufferSize:    100,
		UseFileEvents: true,
	}
}

// NewPoller creates a new Poller with the given configuration.
func NewPoller(config *PollerConfig) (*Poller, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	if config.Interval == 0 {
		config.Interval = 2 * time.Second
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	poller := &Poller{
		path:          config.Path,
		interval:      config.Interval,
		useFileEvents: config.UseFileEvents,
		enableMetrics: config.EnableMetrics,
		ctx:           ctx,
		cancel:        cancel,
		updates:       make(chan *LogEntry, config.BufferSize),
		errChan:       make(chan error, 1),
		done:          make(chan struct{}),
		metrics:       &PollerMetrics{},
	}

	if err := poller.initializePosition(config.ReadFromStart); err != nil {
		cancel()
		return nil, fmt.Errorf("initialize position: %w", err)
	}

	return poller, nil
}

func (p *Poller) initializePosition(readFromStart bool) error {
	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			p.mu.Lock()
			p.lastPos = 0
			p.lastSize = 0
			p.lastMod = time.Time{}
			p.mu.Unlock()
			return nil
		}
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close() //nolint:errcheck
	}()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	var pos int64
	if readFromStart {
		pos = 0
	} else {
		pos, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("seek to end: %w", err)
		}
	}

	p.mu.Lock()
	p.lastPos = pos
	p.lastSize = stat.Size()
	p.lastMod = stat.ModTime()
	p.mu.Unlock()

	return nil
}

// Start begins polling the log file for new entries.
func (p *Poller) Start() <-chan *LogEntry {
	p.runningMu.Lock()
	if p.running {
		p.runningMu.Unlock()
		return p.updates
	}
	p.running = true
	p.runningMu.Unlock()

	go p.poll()

	return p.updates
}

func (p *Poller) poll() {
	defer close(p.done)
	defer close(p.updates)

	if p.useFileEvents {
		if err := p.setupWatcher(); err != nil {
			p.sendError(fmt.Errorf("failed to setup file watcher, falling back to polling: %w", err))
			p.pollWithTimer()
			return
		}
		defer p.cleanupWatcher()
		p.pollWithEvents()
	} else {
		p.pollWithTimer()
	}
}

func (p *Poller) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	p.watcher = watcher
	dir := filepath.Dir(p.path)
	if err := p.watcher.Add(dir); err != nil {
		_ = p.watcher.Close() //nolint:errcheck
		p.watcher = nil
		return fmt.Errorf("watch directory: %w", err)
	}
	return nil
}

func (p *Poller) cleanupWatcher() {
	if p.watcher != nil {
		_ = p.watcher.Close()
		p.watcher = nil
	}
}

func (p *Poller) pollWithEvents() {
	ticker := time.NewTicker(p.interval * 5)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case event, ok := <-p.watcher.Events:
			if !ok {
				return
			}
			switch {
			case event.Has(fsnotify.Write):
				if err := p.checkForUpdates(); err != nil {
					p.sendError(err)
				}
			case event.Has(fsnotify.Create):
				if event.Name == p.path {
					fmt.Printf("[INFO] Log file recreated after rotation: %s\n", event.Name)
					if err := p.checkForUpdates(); err != nil {
						p.sendError(err)
					}
				}
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				fmt.Printf("[INFO] Log file rotation detected (%s event): %s\n", event.Op, event.Name)
				// Drain remaining bytes from the old file while it may still be
				// readable (e.g. on macOS a Rename fires before the path is gone).
				// We must NOT call checkForUpdates() here because it reopens p.path
				// which may already point to the new (empty) file written by MTGA.
				if err := p.drainFile(); err != nil {
					p.sendError(err)
				}
				p.mu.Lock()
				p.lastPos = 0
				p.lastSize = 0
				p.lastMod = time.Time{}
				p.mu.Unlock()
				fmt.Println("[INFO] Position tracking reset, waiting for new log file...")
			}
		case err, ok := <-p.watcher.Errors:
			if !ok {
				return
			}
			p.sendError(fmt.Errorf("watcher error: %w", err))
		case <-ticker.C:
			if err := p.checkForUpdates(); err != nil {
				p.sendError(err)
			}
		}
	}
}

func (p *Poller) pollWithTimer() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.checkForUpdates(); err != nil {
				p.sendError(err)
			}
		}
	}
}

func (p *Poller) sendError(err error) {
	select {
	case p.errChan <- err:
	default:
	}
}

func (p *Poller) checkForUpdates() error {
	start := time.Now()
	var entriesProcessed uint64
	var hadError bool

	defer func() {
		if p.enableMetrics {
			duration := time.Since(start)
			p.metrics.mu.Lock()
			p.metrics.PollCount++
			p.metrics.EntriesProcessed += entriesProcessed
			p.metrics.TotalProcessingTime += duration
			p.metrics.LastPollTime = start
			p.metrics.LastPollDuration = duration
			if p.metrics.PollCount > 0 {
				p.metrics.AverageEntriesPerPoll = float64(p.metrics.EntriesProcessed) / float64(p.metrics.PollCount)
			}
			if hadError {
				p.metrics.ErrorCount++
			}
			p.metrics.mu.Unlock()
		}
	}()

	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			p.mu.Lock()
			p.lastPos = 0
			p.lastSize = 0
			p.lastMod = time.Time{}
			p.mu.Unlock()
			return nil
		}
		hadError = true
		return fmt.Errorf("open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		hadError = true
		_ = file.Close() //nolint:errcheck
		return fmt.Errorf("stat file: %w", err)
	}

	p.mu.RLock()
	lastPos := p.lastPos
	lastSize := p.lastSize
	lastMod := p.lastMod
	p.mu.RUnlock()

	if stat.Size() < lastPos || (stat.Size() < lastSize && !stat.ModTime().Equal(lastMod)) {
		fmt.Printf("[INFO] Log file rotation detected (size decreased from %d to %d bytes)\n", lastSize, stat.Size())
		p.mu.Lock()
		p.lastPos = 0
		p.mu.Unlock()
		lastPos = 0
	}

	if stat.Size() <= lastPos {
		_ = file.Close() //nolint:errcheck
		return nil
	}

	if _, err := file.Seek(lastPos, io.SeekStart); err != nil {
		hadError = true
		_ = file.Close() //nolint:errcheck
		return fmt.Errorf("seek to position %d: %w", lastPos, err)
	}

	scanner := bufio.NewScanner(file)
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	var newEntries []*LogEntry
	newPos := lastPos

	for scanner.Scan() {
		line := scanner.Text()
		entry := &LogEntry{Raw: line}
		entry.parseJSON()
		if entry.IsJSON {
			newEntries = append(newEntries, entry)
			entriesProcessed++
		}
		newPos += int64(len(line)) + 1
	}

	if err := scanner.Err(); err != nil {
		hadError = true
		_ = file.Close() //nolint:errcheck
		return fmt.Errorf("scan file: %w", err)
	}

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err == nil {
		newPos = currentPos
	}

	_ = file.Close() //nolint:errcheck

	p.mu.Lock()
	p.lastPos = newPos
	p.lastSize = stat.Size()
	p.lastMod = stat.ModTime()
	p.mu.Unlock()

	for _, entry := range newEntries {
		select {
		case p.updates <- entry:
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}

	return nil
}

// drainFile reads any remaining lines from p.path starting at p.lastPos and
// sends parsed JSON entries to p.updates. It is used exclusively during a
// Remove/Rename event so that we do not reopen the path via checkForUpdates
// (which might already point to a new file created by MTGA).
// drainFile does NOT update p.lastPos/lastSize/lastMod — the caller resets
// those to zero after draining.
func (p *Poller) drainFile() error {
	p.mu.RLock()
	lastPos := p.lastPos
	p.mu.RUnlock()

	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File already gone — nothing left to drain.
			return nil
		}
		return fmt.Errorf("drain open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("drain stat file: %w", err)
	}

	if stat.Size() <= lastPos {
		return nil
	}

	if _, err := file.Seek(lastPos, io.SeekStart); err != nil {
		return fmt.Errorf("drain seek: %w", err)
	}

	scanner := bufio.NewScanner(file)
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		entry := &LogEntry{Raw: line}
		entry.parseJSON()
		if entry.IsJSON {
			select {
			case p.updates <- entry:
			case <-p.ctx.Done():
				return p.ctx.Err()
			}
		}
	}

	return scanner.Err()
}

// Stop stops the poller and closes the updates channel.
func (p *Poller) Stop() {
	p.runningMu.Lock()
	if !p.running {
		p.runningMu.Unlock()
		return
	}
	p.running = false
	p.runningMu.Unlock()

	p.cancel()
	<-p.done
}

// Errors returns a channel that receives errors encountered during polling.
func (p *Poller) Errors() <-chan error {
	return p.errChan
}

// IsRunning returns whether the poller is currently running.
func (p *Poller) IsRunning() bool {
	p.runningMu.RLock()
	defer p.runningMu.RUnlock()
	return p.running
}

// Metrics returns a copy of the current poller metrics.
func (p *Poller) Metrics() *PollerMetrics {
	if !p.enableMetrics {
		return nil
	}
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	return &PollerMetrics{
		PollCount:             p.metrics.PollCount,
		EntriesProcessed:      p.metrics.EntriesProcessed,
		ErrorCount:            p.metrics.ErrorCount,
		TotalProcessingTime:   p.metrics.TotalProcessingTime,
		LastPollTime:          p.metrics.LastPollTime,
		LastPollDuration:      p.metrics.LastPollDuration,
		AverageEntriesPerPoll: p.metrics.AverageEntriesPerPoll,
	}
}
