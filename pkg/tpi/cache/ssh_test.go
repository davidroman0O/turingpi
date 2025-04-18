package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshTestConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote_dir"`
}

func loadSSHTestConfig(t *testing.T) *SSHConfig {
	configPath := filepath.Join("testdata", "ssh_config.json")
	configFile, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("Failed to open SSH config file: %v", err)
	}
	defer configFile.Close()

	var testConfig sshTestConfig
	if err := json.NewDecoder(configFile).Decode(&testConfig); err != nil {
		t.Fatalf("Failed to decode SSH config: %v", err)
	}

	return &SSHConfig{
		Host:     testConfig.Host,
		Port:     testConfig.Port,
		User:     testConfig.User,
		Password: testConfig.Password,
	}
}

// TestSSHCache performs integration tests with real hardware.
// These tests require SSH access to a real machine.
// Set the following environment variables to run these tests:
// SSH_TEST_HOST: hostname/IP of the test machine
// SSH_TEST_PORT: SSH port (default: 22)
// SSH_TEST_USER: SSH username
// SSH_TEST_PASSWORD: SSH password (optional)
// SSH_TEST_KEY_FILE: path to private key file (optional, but either password or key file must be provided)
// SSH_TEST_DIR: remote directory to use for testing (will be created if doesn't exist)
func TestSSHCache(t *testing.T) {

	config := loadSSHTestConfig(t)
	remoteDir := "/tmp/sshcache_test"

	cache, err := NewSSHCache(*config, remoteDir)
	if err != nil {
		t.Fatalf("Failed to create SSHCache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	t.Run("Put and Get", func(t *testing.T) {
		content := "test content"
		metadata := Metadata{
			Filename:    "test.txt",
			ContentType: "text/plain",
			Size:        int64(len(content)),
			ModTime:     time.Now(),
			Tags:        map[string]string{"type": "test"},
			OSType:      "linux",
			OSVersion:   "5.10",
		}

		// Test Put
		reader := strings.NewReader(content)
		putMeta, err := cache.Put(ctx, "test1", metadata, reader)
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if putMeta.Hash == "" {
			t.Error("Hash should be generated")
		}

		// Test Get with content
		getMeta, getReader, err := cache.Get(ctx, "test1", true)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer getReader.Close()

		if getMeta.Filename != metadata.Filename {
			t.Errorf("Expected filename %s, got %s", metadata.Filename, getMeta.Filename)
		}

		gotContent, err := io.ReadAll(getReader)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}
		if string(gotContent) != content {
			t.Errorf("Expected content %q, got %q", content, string(gotContent))
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := cache.Exists(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist")
		}

		content := "test content"
		metadata := Metadata{Filename: "test.txt"}
		_, err = cache.Put(ctx, "test2", metadata, strings.NewReader(content))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		exists, err = cache.Exists(ctx, "test2")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if !exists {
			t.Error("Key should exist")
		}
	})

	t.Run("List and Delete", func(t *testing.T) {
		// Put items with different tags
		items := []struct {
			key     string
			content string
			tags    map[string]string
		}{
			{"list1", "content1", map[string]string{"type": "doc", "env": "prod"}},
			{"list2", "content2", map[string]string{"type": "img", "env": "prod"}},
		}

		for _, item := range items {
			metadata := Metadata{
				Filename: item.key + ".txt",
				Tags:     item.tags,
			}
			_, err := cache.Put(ctx, item.key, metadata, strings.NewReader(item.content))
			if err != nil {
				t.Fatalf("Put failed for %s: %v", item.key, err)
			}
		}

		// List with tag filter
		prodItems, err := cache.List(ctx, map[string]string{"env": "prod"})
		if err != nil {
			t.Fatalf("List with filter failed: %v", err)
		}
		if len(prodItems) != 2 {
			t.Errorf("Expected 2 prod items, got %d", len(prodItems))
		}

		// Delete one item
		err = cache.Delete(ctx, "list1")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify it's gone
		exists, err := cache.Exists(ctx, "list1")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist after deletion")
		}

		// List again to verify count
		prodItems, err = cache.List(ctx, map[string]string{"env": "prod"})
		if err != nil {
			t.Fatalf("List with filter failed: %v", err)
		}
		if len(prodItems) != 1 {
			t.Errorf("Expected 1 prod item after deletion, got %d", len(prodItems))
		}
	})

	t.Run("Location", func(t *testing.T) {
		loc := cache.Location()
		expectedPrefix := "ssh://root@192.168.1.90"
		if !strings.HasPrefix(loc, expectedPrefix) {
			t.Errorf("Expected location to start with %s, got %s", expectedPrefix, loc)
		}
		if !strings.HasSuffix(loc, remoteDir) {
			t.Errorf("Expected location to end with %s, got %s", remoteDir, loc)
		}
	})
}

func TestSSHCacheNestedFolders(t *testing.T) {
	config := loadSSHTestConfig(t)
	remoteDir := "/tmp/sshcache_test"

	cache, err := NewSSHCache(*config, remoteDir)
	if err != nil {
		t.Fatalf("Failed to create SSHCache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Test nested directory structure
	testCases := []struct {
		key     string
		content string
		meta    Metadata
	}{
		{
			key:     "root/file1",
			content: "root content",
			meta: Metadata{
				OSType:    "linux",
				OSVersion: "ubuntu-20.04",
				Tags:      map[string]string{"arch": "amd64"},
			},
		},
		{
			key:     "folder1/file2",
			content: "nested content 1",
			meta: Metadata{
				OSType:    "darwin",
				OSVersion: "12.0",
				Tags:      map[string]string{"arch": "arm64"},
			},
		},
		{
			key:     "folder1/folder2/file3",
			content: "nested content 2",
			meta: Metadata{
				OSType:    "windows",
				OSVersion: "10",
				Tags:      map[string]string{"arch": "amd64"},
			},
		},
	}

	// Put files in nested directories
	for _, tc := range testCases {
		tc.meta.Key = tc.key // Set the key in metadata
		_, err := cache.Put(ctx, tc.key, tc.meta, strings.NewReader(tc.content))
		if err != nil {
			t.Fatalf("Failed to put file %s: %v", tc.key, err)
		}
	}

	// Verify files can be retrieved
	for _, tc := range testCases {
		meta, reader, err := cache.Get(ctx, tc.key, true)
		if err != nil {
			t.Errorf("Failed to get file %s: %v", tc.key, err)
			continue
		}
		defer reader.Close()

		// Check metadata
		if meta.OSType != tc.meta.OSType || meta.OSVersion != tc.meta.OSVersion {
			t.Errorf("Metadata mismatch for %s: got %v, want %v", tc.key, meta, tc.meta)
		}

		// Check content
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("Failed to read content for %s: %v", tc.key, err)
			continue
		}
		if string(content) != tc.content {
			t.Errorf("Content mismatch for %s: got %q, want %q", tc.key, string(content), tc.content)
		}
	}

	// Test RebuildIndex with nested folders
	if err := cache.RebuildIndex(ctx); err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	// Verify all files are indexed
	for _, tc := range testCases {
		meta, err := cache.Stat(ctx, tc.key)
		if err != nil {
			t.Errorf("Failed to stat %s after index rebuild: %v", tc.key, err)
			continue
		}
		if meta.OSType != tc.meta.OSType || meta.OSVersion != tc.meta.OSVersion {
			t.Errorf("Metadata mismatch after index rebuild for %s: got %v, want %v", tc.key, meta, tc.meta)
		}
	}
}

func TestSSHCacheCleanup(t *testing.T) {
	config := loadSSHTestConfig(t)
	remoteDir := "/tmp/sshcache_test"

	cache, err := NewSSHCache(*config, remoteDir)
	if err != nil {
		t.Fatalf("Failed to create SSHCache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Create a valid file with metadata
	validKey := "valid/file"
	validContent := "valid content"
	validMeta := Metadata{
		Key:       validKey,
		OSType:    "linux",
		OSVersion: "ubuntu-20.04",
		Tags:      map[string]string{"arch": "amd64"},
	}
	_, err = cache.Put(ctx, validKey, validMeta, strings.NewReader(validContent))
	if err != nil {
		t.Fatalf("Failed to put valid file: %v", err)
	}

	// Create orphaned .data files in different directories
	orphanedFiles := []string{
		"orphaned1.data",
		"nested/orphaned2.data",
		"nested/deep/orphaned3.data",
	}

	// Create orphaned files using SSH commands
	for _, file := range orphanedFiles {
		fullPath := filepath.Join(remoteDir, file)
		session, err := cache.client.NewSession()
		if err != nil {
			t.Fatalf("Failed to create SSH session: %v", err)
		}

		// Create parent directory and file
		dirPath := filepath.Dir(fullPath)
		cmd := fmt.Sprintf("mkdir -p %s && echo 'orphaned content' > %s", dirPath, fullPath)
		if err := session.Run(cmd); err != nil {
			session.Close()
			t.Fatalf("Failed to create orphaned file %s: %v", file, err)
		}
		session.Close()
	}

	// Test non-recursive cleanup (should only clean up root level files)
	count, err := cache.Cleanup(ctx, false)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if count != 1 { // Should only clean up root orphaned file
		t.Errorf("Expected 1 file cleaned at root level, got %d", count)
	}

	// Verify root orphaned file is removed but nested ones still exist
	for i, file := range orphanedFiles {
		fullPath := filepath.Join(remoteDir, file)
		session, err := cache.client.NewSession()
		if err != nil {
			t.Fatalf("Failed to create SSH session: %v", err)
		}

		err = session.Run(fmt.Sprintf("test -f %s", fullPath))
		session.Close()

		if i == 0 { // First file should be removed (root level)
			if err == nil {
				t.Error("Root orphaned file should have been removed")
			}
		} else { // Nested files should still exist
			if err != nil {
				t.Errorf("Nested orphaned file %s should still exist", file)
			}
		}
	}

	// Test recursive cleanup (should clean up remaining nested files and empty directories)
	count, err = cache.Cleanup(ctx, true)
	if err != nil {
		t.Fatalf("Recursive cleanup failed: %v", err)
	}
	expectedCount := 4 // 2 nested files + 2 empty directories
	if count != expectedCount {
		t.Errorf("Expected %d items cleaned (2 files + 2 directories), got %d", expectedCount, count)
	}

	// Verify all orphaned files and empty directories are removed
	session, err := cache.client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Check if any of the test directories still exist
	dirsToCheck := []string{
		filepath.Join(remoteDir, "nested"),
		filepath.Join(remoteDir, "nested/deep"),
	}
	for _, dir := range dirsToCheck {
		if err := session.Run(fmt.Sprintf("test -d %s", dir)); err == nil {
			t.Errorf("Directory %s should have been removed", dir)
		}
	}

	// Verify valid file still exists
	_, reader, err := cache.Get(ctx, validKey, true)
	if err != nil {
		t.Errorf("Valid file was removed: %v", err)
	}
	if reader != nil {
		defer reader.Close()
	}
}

// Mock SSH client for testing
type mockSSHClient struct {
	// Add any fields needed for mocking
}

// Mock SSH session for testing
type mockSSHSession struct {
	// Add any fields needed for mocking
}

func (s *mockSSHSession) Run(cmd string) error {
	return nil
}

func (s *mockSSHSession) Start(cmd string) error {
	return nil
}

func (s *mockSSHSession) Wait() error {
	return nil
}

func (s *mockSSHSession) Close() error {
	return nil
}

func (s *mockSSHSession) StdinPipe() (io.WriteCloser, error) {
	return nil, nil
}

func (s *mockSSHSession) StdoutPipe() (io.Reader, error) {
	return nil, nil
}

func (s *mockSSHSession) StderrPipe() (io.Reader, error) {
	return nil, nil
}

func (m *mockSSHClient) NewSession() (*ssh.Session, error) {
	return nil, nil // Mock implementation
}

func TestSSHCacheVerifyIntegrity(t *testing.T) {
	cfg := SSHConfig{
		Host:     "localhost",
		Port:     22,
		User:     "test",
		Password: "test",
	}
	cacheDir := "/tmp/sshcache_test"
	cache, err := NewSSHCache(cfg, cacheDir)
	if err != nil {
		t.Fatalf("Failed to create SSHCache: %v", err)
	}
	defer cache.Close()

	// Test putting a file
	content := "test content"
	meta := Metadata{
		Key:         "test-key",
		Filename:    "test.txt",
		ContentType: "text/plain",
		Size:        int64(len(content)),
		ModTime:     time.Now(),
		Tags:        map[string]string{"type": "test"},
		OSType:      "linux",
		OSVersion:   "5.10",
	}
	_, err = cache.Put(context.Background(), "test-key", meta, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to put file: %v", err)
	}

	tests := []struct {
		name          string
		setup         func() error
		expectedCount int
		expectedErr   bool
	}{
		{
			name: "orphaned data file",
			setup: func() error {
				session, err := cache.client.NewSession()
				if err != nil {
					return fmt.Errorf("failed to create SSH session: %w", err)
				}
				defer session.Close()

				// Create orphaned data file
				cmd := fmt.Sprintf("mkdir -p %s/test && echo 'test data' > %s/test/orphaned.data", cacheDir, cacheDir)
				if err := session.Run(cmd); err != nil {
					return fmt.Errorf("failed to create orphaned data file: %w", err)
				}
				return nil
			},
			expectedCount: 1,
			expectedErr:   false,
		},
		{
			name: "orphaned meta file",
			setup: func() error {
				session, err := cache.client.NewSession()
				if err != nil {
					return fmt.Errorf("failed to create SSH session: %w", err)
				}
				defer session.Close()

				// Create orphaned meta file
				cmd := fmt.Sprintf("mkdir -p %s/test && echo 'test metadata' > %s/test/orphaned.meta", cacheDir, cacheDir)
				if err := session.Run(cmd); err != nil {
					return fmt.Errorf("failed to create orphaned meta file: %w", err)
				}
				return nil
			},
			expectedCount: 1,
			expectedErr:   false,
		},
		{
			name: "corrupted metadata",
			setup: func() error {
				session, err := cache.client.NewSession()
				if err != nil {
					return fmt.Errorf("failed to create SSH session: %w", err)
				}
				defer session.Close()

				// Create data and corrupted meta files
				cmds := []string{
					fmt.Sprintf("mkdir -p %s/test", cacheDir),
					fmt.Sprintf("echo 'test data' > %s/test/corrupted.data", cacheDir),
					fmt.Sprintf("echo 'invalid json' > %s/test/corrupted.meta", cacheDir),
				}
				cmd := strings.Join(cmds, " && ")
				if err := session.Run(cmd); err != nil {
					return fmt.Errorf("failed to create corrupted files: %w", err)
				}
				return nil
			},
			expectedCount: 1,
			expectedErr:   false,
		},
		{
			name: "no issues",
			setup: func() error {
				// Create valid data and meta files using the cache's Put method
				content := "test data"
				meta := Metadata{
					Key:       "test/valid",
					OSType:    "linux",
					OSVersion: "ubuntu-20.04",
					Tags:      map[string]string{"arch": "amd64"},
				}
				_, err := cache.Put(context.Background(), "test/valid", meta, strings.NewReader(content))
				if err != nil {
					return fmt.Errorf("failed to create valid files: %w", err)
				}
				return nil
			},
			expectedCount: 0,
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any previous test files
			session, err := cache.client.NewSession()
			if err != nil {
				t.Fatalf("failed to create cleanup session: %v", err)
			}
			session.Run(fmt.Sprintf("rm -rf %s/test", cacheDir))
			session.Close()

			// Setup test files
			if err := tt.setup(); err != nil {
				t.Fatalf("failed to setup test: %v", err)
			}

			issues, err := cache.VerifyIntegrity(context.Background())
			if (err != nil) != tt.expectedErr {
				t.Errorf("VerifyIntegrity() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}
			if len(issues) != tt.expectedCount {
				t.Errorf("VerifyIntegrity() count = %v, want %v, issues: %v", len(issues), tt.expectedCount, issues)
			}
		})
	}
}
