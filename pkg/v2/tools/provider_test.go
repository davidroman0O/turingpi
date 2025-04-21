package tools_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// sshConfig holds the SSH configuration for testing
type sshConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote_dir"`
}

// loadSSHConfig loads the SSH configuration from the test file
func loadSSHConfig(t *testing.T) *sshConfig {
	// Read the SSH config file
	configPath := filepath.Join("../cache/testdata", "ssh_config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read SSH config file: %v", err)
	}

	var config sshConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse SSH config: %v", err)
	}

	return &config
}

// TestTuringPiToolProvider_Caches tests both local and remote cache functionality
func TestTuringPiToolProvider_Caches(t *testing.T) {
	// Create a temporary directory for the local cache
	tempDir, err := os.MkdirTemp("", "turingpi_test_cache_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Load SSH configuration for remote cache
	sshConfig := loadSSHConfig(t)

	// Create a mock BMC executor for the node tool
	mockBMC := &mockBMCExecutor{}

	// Create the tool provider configuration
	providerConfig := &tools.TuringPiToolConfig{
		BMCExecutor: mockBMC,
		CacheDir:    tempDir,
		RemoteCache: &tools.RemoteCacheConfig{
			Host:       sshConfig.Host,
			User:       sshConfig.User,
			Password:   sshConfig.Password,
			RemotePath: sshConfig.RemoteDir,
		},
	}

	// Create the tool provider
	provider, err := tools.NewTuringPiToolProvider(providerConfig)
	if err != nil {
		t.Fatalf("Failed to create tool provider: %v", err)
	}

	// Test the local cache
	t.Run("LocalCache", func(t *testing.T) {
		// Get the local cache
		localCache := provider.GetLocalCache()
		if localCache == nil {
			t.Fatal("Local cache is nil")
		}

		// Check that the cache directory is correct
		if localCache.Location() != tempDir {
			t.Errorf("Expected local cache location %s, got %s", tempDir, localCache.Location())
		}

		// Test basic cache operations
		ctx := context.Background()

		// Create test data
		testKey := "test-local-key"
		testContent := "This is local test content"
		metadata := cache.Metadata{
			Key:      testKey,
			Filename: "test.txt",
			Tags:     map[string]string{"test": "true", "type": "local"},
		}

		// Store in cache
		reader := strings.NewReader(testContent)
		_, err := localCache.Put(ctx, testKey, metadata, reader)
		if err != nil {
			t.Fatalf("Failed to put data in local cache: %v", err)
		}

		// Verify existence
		exists, err := localCache.Exists(ctx, testKey)
		if err != nil {
			t.Errorf("Failed to check existence in local cache: %v", err)
		}
		if !exists {
			t.Error("Data should exist in local cache but doesn't")
		}

		// Retrieve data
		_, dataReader, err := localCache.Get(ctx, testKey, true)
		if err != nil {
			t.Fatalf("Failed to get data from local cache: %v", err)
		}
		defer dataReader.Close()

		// Read content
		content, err := io.ReadAll(dataReader)
		if err != nil {
			t.Fatalf("Failed to read content from local cache: %v", err)
		}

		// Verify content
		if string(content) != testContent {
			t.Errorf("Expected content %q, got %q", testContent, string(content))
		}

		// Cleanup
		err = localCache.Delete(ctx, testKey)
		if err != nil {
			t.Errorf("Failed to delete from local cache: %v", err)
		}
	})

	// Skip remote cache test if SSH is not available
	// This allows tests to run in CI environments without SSH
	if os.Getenv("SKIP_SSH_TESTS") == "true" {
		t.Skip("Skipping remote cache tests because SKIP_SSH_TESTS=true")
	}

	// Test the remote cache
	t.Run("RemoteCache", func(t *testing.T) {
		// Get the remote cache
		remoteCache := provider.GetRemoteCache()
		if remoteCache == nil {
			t.Fatal("Remote cache is nil")
		}

		// Check that the remote path is in the location string (exact format may vary)
		location := remoteCache.Location()
		if !strings.Contains(location, sshConfig.RemoteDir) {
			t.Errorf("Expected location to contain %s, got %s", sshConfig.RemoteDir, location)
		}

		// Test basic cache operations
		ctx := context.Background()

		// Create test data
		testKey := "test-remote-key-" + time.Now().Format("20060102150405")
		testContent := "This is remote test content"
		metadata := cache.Metadata{
			Key:      testKey,
			Filename: "test.txt",
			Tags:     map[string]string{"test": "true", "type": "remote"},
		}

		// Store in cache
		reader := strings.NewReader(testContent)
		_, err := remoteCache.Put(ctx, testKey, metadata, reader)
		if err != nil {
			t.Fatalf("Failed to put data in remote cache: %v", err)
		}

		// Verify existence
		exists, err := remoteCache.Exists(ctx, testKey)
		if err != nil {
			t.Errorf("Failed to check existence in remote cache: %v", err)
		}
		if !exists {
			t.Error("Data should exist in remote cache but doesn't")
		}

		// Retrieve data
		_, dataReader, err := remoteCache.Get(ctx, testKey, true)
		if err != nil {
			t.Fatalf("Failed to get data from remote cache: %v", err)
		}
		defer dataReader.Close()

		// Read content
		content, err := io.ReadAll(dataReader)
		if err != nil {
			t.Fatalf("Failed to read content from remote cache: %v", err)
		}

		// Verify content
		if string(content) != testContent {
			t.Errorf("Expected content %q, got %q", testContent, string(content))
		}

		// Cleanup
		err = remoteCache.Delete(ctx, testKey)
		if err != nil {
			t.Errorf("Failed to delete from remote cache: %v", err)
		}
	})
}

// mockBMCExecutor is a mock implementation of bmc.CommandExecutor for testing
type mockBMCExecutor struct{}

func (m *mockBMCExecutor) ExecuteCommand(cmd string) (stdout string, stderr string, err error) {
	return "mock output", "", nil
}

// TestTuringPiToolProvider_InitializationErrors tests handling of initialization errors
func TestTuringPiToolProvider_InitializationErrors(t *testing.T) {
	// Test with invalid local cache directory
	t.Run("InvalidLocalCacheDir", func(t *testing.T) {
		providerConfig := &tools.TuringPiToolConfig{
			CacheDir: "/nonexistent/dir/that/should/not/exist",
		}

		_, err := tools.NewTuringPiToolProvider(providerConfig)
		if err == nil {
			t.Error("Expected error with invalid cache directory, got nil")
		}
	})

	// Test with invalid remote cache configuration
	t.Run("InvalidRemoteCache", func(t *testing.T) {
		// Create a temporary directory for the local cache
		tempDir, err := os.MkdirTemp("", "turingpi_test_cache_*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		providerConfig := &tools.TuringPiToolConfig{
			BMCExecutor: &mockBMCExecutor{},
			CacheDir:    tempDir,
			RemoteCache: &tools.RemoteCacheConfig{
				Host:       "nonexistent.host.invalid",
				User:       "invalid",
				Password:   "invalid",
				RemotePath: "/invalid/path",
			},
		}

		// This should not cause NewTuringPiToolProvider to fail, it should continue without remote cache
		provider, err := tools.NewTuringPiToolProvider(providerConfig)
		if err != nil {
			t.Errorf("Unexpected error with invalid remote cache: %v", err)
		}

		if provider.GetRemoteCache() != nil {
			t.Error("Expected remote cache to be nil with invalid configuration")
		}
	})
}
