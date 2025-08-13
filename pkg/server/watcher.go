package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// BinaryWatcher monitors the server binary for changes
type BinaryWatcher struct {
	coordinator *UpgradeCoordinator
	watcher     *fsnotify.Watcher
	binaryPath  string
	checksum    [32]byte
	mu          sync.RWMutex
	stopChan    chan struct{}
	stopped     sync.WaitGroup

	// Configuration
	debounceDelay time.Duration
	cooldownDelay time.Duration
	lastUpgrade   time.Time
}

// NewBinaryWatcher creates a new binary watcher
func NewBinaryWatcher(coordinator *UpgradeCoordinator) (*BinaryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to resolve binary path: %w", err)
	}

	// Calculate initial checksum
	checksum, err := calculateChecksum(binaryPath)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to calculate initial checksum: %w", err)
	}

	bw := &BinaryWatcher{
		coordinator:   coordinator,
		watcher:       watcher,
		binaryPath:    binaryPath,
		checksum:      checksum,
		stopChan:      make(chan struct{}),
		debounceDelay: 2 * time.Second,  // Wait for file write to complete
		cooldownDelay: 10 * time.Second, // Minimum time between upgrades
	}

	// Watch the binary file
	if err := watcher.Add(binaryPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch binary: %w", err)
	}

	// Also watch the directory for atomic renames
	binaryDir := filepath.Dir(binaryPath)
	if err := watcher.Add(binaryDir); err != nil {
		log.Printf("Warning: failed to watch binary directory %s: %v", binaryDir, err)
	}

	return bw, nil
}

// Start begins watching for binary changes
func (bw *BinaryWatcher) Start(ctx context.Context) {
	bw.stopped.Add(1)
	go bw.watchLoop(ctx)
	log.Printf("Binary watcher started for %s", bw.binaryPath)
}

// Stop stops the binary watcher
func (bw *BinaryWatcher) Stop() {
	close(bw.stopChan)
	bw.watcher.Close()
	bw.stopped.Wait()
	log.Println("Binary watcher stopped")
}

// watchLoop is the main event loop for watching binary changes
func (bw *BinaryWatcher) watchLoop(ctx context.Context) {
	defer bw.stopped.Done()

	var debounceTimer *time.Timer
	var pendingEvent *fsnotify.Event

	for {
		select {
		case <-ctx.Done():
			return

		case <-bw.stopChan:
			return

		case event, ok := <-bw.watcher.Events:
			if !ok {
				return
			}

			// Filter events for our binary
			if !bw.isRelevantEvent(event) {
				continue
			}

			log.Printf("Binary change detected: %s (%s)", event.Name, event.Op)

			// Store the event and start/reset debounce timer
			pendingEvent = &event
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(bw.debounceDelay, func() {
				bw.handleBinaryChange(ctx, pendingEvent)
			})

		case err, ok := <-bw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Binary watcher error: %v", err)
		}
	}
}

// isRelevantEvent checks if the event is relevant to our binary
func (bw *BinaryWatcher) isRelevantEvent(event fsnotify.Event) bool {
	// Check if it's our binary file
	if event.Name == bw.binaryPath {
		return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0
	}

	// Check if it's a new file in the same directory (atomic rename case)
	if filepath.Dir(event.Name) == filepath.Dir(bw.binaryPath) {
		// Check if the new file has the same base name pattern
		baseName := filepath.Base(bw.binaryPath)
		eventBase := filepath.Base(event.Name)

		// Handle atomic updates (e.g., binary.new -> binary)
		if eventBase == baseName || eventBase == baseName+".new" {
			return event.Op&(fsnotify.Create|fsnotify.Rename) != 0
		}
	}

	return false
}

// handleBinaryChange processes a detected binary change
func (bw *BinaryWatcher) handleBinaryChange(ctx context.Context, event *fsnotify.Event) {
	// Check cooldown period
	bw.mu.RLock()
	timeSinceLastUpgrade := time.Since(bw.lastUpgrade)
	bw.mu.RUnlock()

	if timeSinceLastUpgrade < bw.cooldownDelay {
		log.Printf("Skipping upgrade: cooldown period active (%.1fs remaining)",
			(bw.cooldownDelay - timeSinceLastUpgrade).Seconds())
		return
	}

	// Wait a bit for file write to complete
	time.Sleep(500 * time.Millisecond)

	// Verify the binary has actually changed
	newChecksum, err := calculateChecksum(bw.binaryPath)
	if err != nil {
		log.Printf("Failed to calculate new checksum: %v", err)
		return
	}

	bw.mu.RLock()
	oldChecksum := bw.checksum
	bw.mu.RUnlock()

	if newChecksum == oldChecksum {
		log.Println("Binary unchanged (same checksum), skipping upgrade")
		return
	}

	// Verify the new binary is valid
	if err := bw.verifyBinary(bw.binaryPath); err != nil {
		log.Printf("Binary validation failed: %v", err)
		return
	}

	log.Printf("Binary changed (checksum: %x -> %x), triggering upgrade",
		oldChecksum[:8], newChecksum[:8])

	// Update checksum
	bw.mu.Lock()
	bw.checksum = newChecksum
	bw.lastUpgrade = time.Now()
	bw.mu.Unlock()

	// Trigger the upgrade
	upgradeCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := bw.coordinator.TriggerUpgrade(upgradeCtx); err != nil {
		log.Printf("Automatic upgrade failed: %v", err)
		// Reset last upgrade time on failure to allow retry sooner
		bw.mu.Lock()
		bw.lastUpgrade = time.Now().Add(-bw.cooldownDelay / 2)
		bw.mu.Unlock()
	}
}

// verifyBinary performs basic validation on the new binary
func (bw *BinaryWatcher) verifyBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat binary: %w", err)
	}

	// Check it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}

	// Check it's executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("not executable")
	}

	// Check minimum size (avoid empty or truncated files)
	if info.Size() < 1024*1024 { // 1MB minimum
		return fmt.Errorf("binary too small (%d bytes)", info.Size())
	}

	// Optional: Check binary signature/version
	// This could involve running the binary with --version flag
	// and verifying the output format

	return nil
}

// calculateChecksum calculates SHA256 checksum of a file
func calculateChecksum(path string) ([32]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return [32]byte{}, err
	}

	var checksum [32]byte
	copy(checksum[:], hasher.Sum(nil))
	return checksum, nil
}

// SetDebounceDelay sets the debounce delay for file change events
func (bw *BinaryWatcher) SetDebounceDelay(delay time.Duration) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.debounceDelay = delay
}

// SetCooldownDelay sets the minimum time between upgrades
func (bw *BinaryWatcher) SetCooldownDelay(delay time.Duration) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.cooldownDelay = delay
}

// GetChecksum returns the current binary checksum
func (bw *BinaryWatcher) GetChecksum() [32]byte {
	bw.mu.RLock()
	defer bw.mu.RUnlock()
	return bw.checksum
}
