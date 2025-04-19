package imageops

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TempDirManager handles the lifecycle of temporary directories used in image operations
type TempDirManager struct {
	baseDir     string
	activeDirs  map[string]time.Time
	mu          sync.RWMutex
	maxAge      time.Duration
	initialized bool
}

var (
	globalTempManager *TempDirManager
	tempManagerOnce   sync.Once
)

// GetTempManager returns the global temporary directory manager
func GetTempManager() *TempDirManager {
	tempManagerOnce.Do(func() {
		globalTempManager = &TempDirManager{
			activeDirs: make(map[string]time.Time),
			maxAge:     24 * time.Hour, // Default max age for temp dirs
		}
	})
	return globalTempManager
}

// Initialize sets up the temporary directory manager with the base directory
func (m *TempDirManager) Initialize(baseDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// Validate and create base directory if needed
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base temp directory: %w", err)
	}

	// Verify directory is writable
	testFile := filepath.Join(baseDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("temp directory is not writable: %w", err)
	}
	os.Remove(testFile)

	m.baseDir = baseDir
	m.initialized = true

	// Start cleanup goroutine
	go m.cleanupRoutine()

	log.Printf("Temporary directory manager initialized with base directory: %s", baseDir)
	return nil
}

// CreateTempDir creates a new temporary directory with the given prefix
func (m *TempDirManager) CreateTempDir(prefix string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return "", fmt.Errorf("temp directory manager not initialized")
	}

	// Create unique temporary directory
	tempDir, err := os.MkdirTemp(m.baseDir, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Register the directory
	m.activeDirs[tempDir] = time.Now()
	log.Printf("Created temporary directory: %s", tempDir)

	return tempDir, nil
}

// CleanupDir removes a temporary directory and its contents
func (m *TempDirManager) CleanupDir(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return fmt.Errorf("temp directory manager not initialized")
	}

	// Check if directory is managed by us
	if _, exists := m.activeDirs[dir]; !exists {
		return fmt.Errorf("directory %s is not managed by temp manager", dir)
	}

	// Remove directory and unregister it
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove temp directory %s: %w", dir, err)
	}

	delete(m.activeDirs, dir)
	log.Printf("Cleaned up temporary directory: %s", dir)
	return nil
}

// cleanupRoutine periodically cleans up old temporary directories
func (m *TempDirManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupOldDirs()
	}
}

// cleanupOldDirs removes directories that are older than maxAge
func (m *TempDirManager) cleanupOldDirs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for dir, created := range m.activeDirs {
		if now.Sub(created) > m.maxAge {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("Warning: Failed to remove old temp directory %s: %v", dir, err)
				continue
			}
			delete(m.activeDirs, dir)
			log.Printf("Cleaned up old temporary directory: %s", dir)
		}
	}
}

// SetMaxAge sets the maximum age for temporary directories
func (m *TempDirManager) SetMaxAge(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxAge = duration
}

// GetActiveDirs returns a list of active temporary directories
func (m *TempDirManager) GetActiveDirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dirs := make([]string, 0, len(m.activeDirs))
	for dir := range m.activeDirs {
		dirs = append(dirs, dir)
	}
	return dirs
}

// Shutdown cleans up all temporary directories and stops the cleanup routine
func (m *TempDirManager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for dir := range m.activeDirs {
		if err := os.RemoveAll(dir); err != nil {
			lastErr = fmt.Errorf("failed to remove temp directory %s: %w", dir, err)
			log.Printf("Warning: %v", lastErr)
		}
		delete(m.activeDirs, dir)
	}

	m.initialized = false
	return lastErr
}
