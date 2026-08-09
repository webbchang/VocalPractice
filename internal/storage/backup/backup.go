package backup

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vocal-practice-app/internal/storage"
)

// ErrNoBackupFile is returned when Restore is called but no backup file exists
// on disk yet.
var ErrNoBackupFile = errors.New("no backup file found")

// Manager persists the state of an in-memory store to a local file on disk so
// that data is not lost across process restarts, and restores it back on start
// up.
//
// The manager keeps the in-memory store as the system of record: every
// mutating write to the store triggers a "dirty" flag (via a callback
// registered on the store). A background goroutine then flushes the dirty
// state to disk. This means local storage is updated when in-memory storage
// changes, without blocking writes on synchronous disk I/O.
type Manager struct {
	store    *storage.MemoryStore
	filePath string

	mu     sync.Mutex
	dirty  bool
	signal chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewManager creates a Manager for the given in-memory store. The provided
// filePath is the local file used as the persistent backup store. The file is
// created on the first backup; until then Restore returns ErrNoBackupFile.
func NewManager(store *storage.MemoryStore, filePath string) *Manager {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}
	m := &Manager{
		store:    store,
		filePath: abs,
		signal:   make(chan struct{}, 1),
	}
	// React to in-memory writes: mark the backup dirty and wake the flusher.
	store.SetWriteCallback(m.markDirty)
	return m
}

// markDirty is the write callback registered on the store. It is safe to call
// while the store lock is held because it only touches the Manager's own
// mutex and never acquires the store lock.
func (m *Manager) markDirty() {
	m.mu.Lock()
	m.dirty = true
	m.mu.Unlock()
	// Non-blocking signal: coalesced by the flusher.
	select {
	case m.signal <- struct{}{}:
	default:
	}
}

func (m *Manager) isDirty() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dirty
}

func (m *Manager) setClean() {
	m.mu.Lock()
	m.dirty = false
	m.mu.Unlock()
}

// Backup writes the current in-memory store state to the local file. It uses
// an atomic temp-file rename so the backup file is never partially written.
func (m *Manager) Backup() error {
	if m.store == nil {
		return errors.New("no store attached")
	}
	data, err := m.store.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	tmp := m.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("write backup file: %w", err)
	}
	if err := os.Rename(tmp, m.filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("finalize backup file: %w", err)
	}
	m.setClean()
	return nil
}

// Restore loads state from the local file into the in-memory store, replacing
// whatever is currently held in memory. It returns ErrNoBackupFile if the file
// does not exist yet (e.g. first run).
func (m *Manager) Restore() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrNoBackupFile, m.filePath)
		}
		return fmt.Errorf("read backup file: %w", err)
	}
	if err := m.store.RestoreFromJSON(data); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}
	log.Printf("[backup] restored in-memory state from %s", m.filePath)
	return nil
}

// HasBackup reports whether a backup file exists on disk.
func (m *Manager) HasBackup() bool {
	if _, err := os.Stat(m.filePath); err == nil {
		return true
	}
	return false
}

// StartAutoBackup launches a background goroutine that persists dirty in-memory
// state to disk. It flushes shortly after a change is detected (via the write
// callback) and also periodically on the given interval as a safety net.
func (m *Manager) StartAutoBackup(interval time.Duration) {
	m.mu.Lock()
	if m.done != nil {
		m.mu.Unlock()
		return // already running
	}
	if interval <= 0 {
		interval = time.Minute
	}
	m.done = make(chan struct{})
	m.mu.Unlock()

	m.wg.Add(1)
	go m.run(interval)
}

func (m *Manager) run(interval time.Duration) {
	defer m.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.done:
			if m.isDirty() {
				if err := m.Backup(); err != nil {
					log.Printf("[backup] final flush failed: %v", err)
				}
			}
			return
		case <-m.signal:
			if err := m.Backup(); err != nil {
				log.Printf("[backup] auto backup failed: %v", err)
			}
		case <-ticker.C:
			if m.isDirty() {
				if err := m.Backup(); err != nil {
					log.Printf("[backup] periodic backup failed: %v", err)
				}
			}
		}
	}
}

// Flush forces an immediate backup of the current in-memory state, regardless
// of whether anything changed.
func (m *Manager) Flush() error {
	return m.Backup()
}

// FilePath returns the configured local backup file path.
func (m *Manager) FilePath() string {
	return m.filePath
}

// Close stops the background flusher and performs a final backup if there are
// pending changes. It is safe to call multiple times.
func (m *Manager) Close() error {
	m.mu.Lock()
	done := m.done
	m.mu.Unlock()
	if done == nil {
		// Not started; just flush if dirty.
		if m.isDirty() {
			return m.Backup()
		}
		return nil
	}
	close(done)
	m.wg.Wait()
	if m.isDirty() {
		return m.Backup()
	}
	return nil
}
