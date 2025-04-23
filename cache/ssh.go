package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHCache implements Cache interface using a remote SSH connection
type SSHCache struct {
	client     *ssh.Client
	remoteDir  string
	sftpClient *sftp.Client
	index      *Index
	indexMgr   *IndexManager
	mu         sync.RWMutex
}

// SSHConfig holds configuration for SSH connection
type SSHConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyFile  string
}

// NewSSHCache creates a new SSH-based cache
func NewSSHCache(config SSHConfig, remoteDir string) (*SSHCache, error) {
	var authMethods []ssh.AuthMethod

	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	if config.KeyFile != "" {
		key, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Replace with proper host key verification
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	// Ensure remote directory exists
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		sftpClient.Close()
		client.Close()
		return nil, fmt.Errorf("failed to create remote directory: %w", err)
	}

	cache := &SSHCache{
		client:     client,
		remoteDir:  remoteDir,
		sftpClient: sftpClient,
		index:      NewIndex(),
	}

	// Create index manager with 5-minute refresh interval
	cache.indexMgr = NewIndexManager(cache, 5*time.Minute)
	if err := cache.indexMgr.Start(context.Background()); err != nil {
		cache.Close()
		return nil, fmt.Errorf("failed to start index manager: %w", err)
	}

	return cache, nil
}

func (c *SSHCache) Close() error {
	if c.indexMgr != nil {
		c.indexMgr.Stop()
	}
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *SSHCache) getMetadataPath(key string) string {
	return filepath.Join(c.remoteDir, key+".meta")
}

func (c *SSHCache) getContentPath(key string) string {
	return filepath.Join(c.remoteDir, key+".data")
}

func (c *SSHCache) Put(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Create content file
	contentPath := c.getContentPath(key)

	// Create parent directories if they don't exist
	parentDir := filepath.Dir(contentPath)
	if err := c.sftpClient.MkdirAll(parentDir); err != nil {
		return nil, fmt.Errorf("failed to create parent directories: %w", err)
	}

	contentFile, err := c.sftpClient.Create(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote content file: %w", err)
	}
	defer contentFile.Close()

	// Copy content and calculate hash
	if reader != nil {
		// Create a TeeReader to calculate hash while copying
		hash := sha256.New()
		teeReader := io.TeeReader(reader, hash)

		if _, err := io.Copy(contentFile, teeReader); err != nil {
			c.sftpClient.Remove(contentPath)
			return nil, fmt.Errorf("failed to write content: %w", err)
		}

		if metadata.Hash == "" {
			metadata.Hash = hex.EncodeToString(hash.Sum(nil))
		}
	}

	// Write metadata
	metadataPath := c.getMetadataPath(key)

	// Create parent directories for metadata file if they don't exist
	parentDir = filepath.Dir(metadataPath)
	if err := c.sftpClient.MkdirAll(parentDir); err != nil {
		c.sftpClient.Remove(contentPath)
		return nil, fmt.Errorf("failed to create parent directories for metadata: %w", err)
	}

	metadataFile, err := c.sftpClient.Create(metadataPath)
	if err != nil {
		c.sftpClient.Remove(contentPath)
		return nil, fmt.Errorf("failed to create remote metadata file: %w", err)
	}
	defer metadataFile.Close()

	if err := json.NewEncoder(metadataFile).Encode(metadata); err != nil {
		c.sftpClient.Remove(contentPath)
		c.sftpClient.Remove(metadataPath)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update index
	metadata.Key = key
	c.index.updateIndex(&metadata)

	return &metadata, nil
}

func (c *SSHCache) Get(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	// Read metadata
	metadata, err := c.Stat(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	if !getContent {
		return metadata, nil, nil
	}

	// Open content file
	contentPath := c.getContentPath(key)
	content, err := c.sftpClient.Open(contentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open remote content file: %w", err)
	}

	return metadata, content, nil
}

func (c *SSHCache) Stat(ctx context.Context, key string) (*Metadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read metadata file
	metadataPath := c.getMetadataPath(key)
	metadataFile, err := c.sftpClient.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote metadata file: %w", err)
	}
	defer metadataFile.Close()

	var metadata Metadata
	if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return &metadata, nil
}

func (c *SSHCache) Exists(ctx context.Context, key string) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	_, err := c.sftpClient.Stat(c.getMetadataPath(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *SSHCache) List(ctx context.Context, filterTags map[string]string) ([]Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use index for efficient filtering
	var results []Metadata
	if len(filterTags) == 0 {
		// Return all items
		results = make([]Metadata, 0, len(c.index.Items))
		for _, meta := range c.index.Items {
			results = append(results, *meta)
		}
		return results, nil
	}

	// Find intersection of all tag filters
	var matchingKeys map[string]bool
	first := true

	for tagKey, tagValue := range filterTags {
		if tagMap, ok := c.index.TagIndex[tagKey]; ok {
			if keys, ok := tagMap[tagValue]; ok {
				if first {
					matchingKeys = make(map[string]bool)
					for _, key := range keys {
						matchingKeys[key] = true
					}
					first = false
				} else {
					// Intersect with existing matches
					newMatches := make(map[string]bool)
					for _, key := range keys {
						if matchingKeys[key] {
							newMatches[key] = true
						}
					}
					matchingKeys = newMatches
				}
			} else {
				return nil, nil // No matches for this tag value
			}
		} else {
			return nil, nil // No matches for this tag key
		}
	}

	// Convert matching keys to metadata list
	results = make([]Metadata, 0, len(matchingKeys))
	for key := range matchingKeys {
		if meta, ok := c.index.Items[key]; ok {
			results = append(results, *meta)
		}
	}

	return results, nil
}

func (c *SSHCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Remove both metadata and content files
	metadataPath := c.getMetadataPath(key)
	contentPath := c.getContentPath(key)

	if err := c.sftpClient.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove remote metadata file: %w", err)
	}

	if err := c.sftpClient.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove remote content file: %w", err)
	}

	// Update index
	c.index.removeFromIndex(key)

	return nil
}

func (c *SSHCache) Location() string {
	return fmt.Sprintf("ssh://%s@%s:%s", c.client.User(), c.client.RemoteAddr(), c.remoteDir)
}

func (c *SSHCache) GetIndex(ctx context.Context) (*Index, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return c.index, nil
}

func (c *SSHCache) RebuildIndex(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	newIndex := NewIndex()

	// Walk through all directories recursively
	walker := c.sftpClient.Walk(c.remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue // Skip entries with errors
		}

		path := walker.Path()
		if walker.Stat().IsDir() {
			continue // Skip directories
		}

		// Only process .meta files
		if filepath.Ext(path) == ".meta" {
			// Get relative path from remote directory
			relPath, err := filepath.Rel(c.remoteDir, path)
			if err != nil {
				continue // Skip invalid paths
			}

			// Extract key by removing .meta extension
			key := strings.TrimSuffix(relPath, ".meta")

			// Read metadata file directly to avoid lock contention
			metadataFile, err := c.sftpClient.Open(path)
			if err != nil {
				continue // Skip invalid entries
			}

			var metadata Metadata
			err = json.NewDecoder(metadataFile).Decode(&metadata)
			metadataFile.Close()
			if err != nil {
				continue // Skip invalid entries
			}

			metadata.Key = key
			newIndex.updateIndex(&metadata)
		}
	}

	c.index = newIndex
	return nil
}

func (c *SSHCache) Cleanup(ctx context.Context, recursive bool) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	cleanedCount := 0

	// List all files in the cache directory
	var cmd string
	if recursive {
		cmd = fmt.Sprintf("find %s -type f -name '*.data'", c.remoteDir)
	} else {
		cmd = fmt.Sprintf("find %s -maxdepth 1 -type f -name '*.data'", c.remoteDir)
	}

	session, err := c.client.NewSession()
	if err != nil {
		return 0, fmt.Errorf("failed to create SSH session: %w", err)
	}
	output, err := session.Output(cmd)
	session.Close()
	if err != nil {
		return 0, fmt.Errorf("failed to list files: %w", err)
	}

	// Process each .data file
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range files {
		if file == "" {
			continue
		}

		// Check if corresponding .meta file exists
		metaFile := strings.TrimSuffix(file, ".data") + ".meta"
		session, err := c.client.NewSession()
		if err != nil {
			return cleanedCount, fmt.Errorf("failed to create SSH session: %w", err)
		}

		// Use test command to check if meta file exists
		err = session.Run(fmt.Sprintf("test -f %s", metaFile))
		session.Close()

		if err != nil {
			// Meta file doesn't exist, remove the orphaned data file
			session, err := c.client.NewSession()
			if err != nil {
				return cleanedCount, fmt.Errorf("failed to create SSH session: %w", err)
			}

			err = session.Run(fmt.Sprintf("rm %s", file))
			session.Close()

			if err != nil {
				return cleanedCount, fmt.Errorf("failed to remove orphaned file %s: %w", file, err)
			}
			cleanedCount++
		}
	}

	// Clean up empty directories if recursive is true
	if recursive {
		session, err := c.client.NewSession()
		if err != nil {
			return cleanedCount, fmt.Errorf("failed to create SSH session: %w", err)
		}

		// Find all empty directories and remove them
		// -mindepth 1 ensures we don't try to remove the root cache directory
		// -depth ensures we process deepest directories first
		cmd = fmt.Sprintf(`find %s -mindepth 1 -depth -type d -empty -exec rm -rf {} \; -exec echo {} \;`, c.remoteDir)
		output, err := session.Output(cmd)
		session.Close()
		if err != nil {
			// Don't fail if directory cleanup fails
			return cleanedCount, nil
		}

		// Count removed directories
		removedDirs := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, dir := range removedDirs {
			if dir != "" {
				cleanedCount++
			}
		}
	}

	return cleanedCount, nil
}

func (c *SSHCache) VerifyIntegrity(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var issues []string

	// Walk through all files recursively
	walker := c.sftpClient.Walk(c.remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue // Skip entries with errors
		}

		path := walker.Path()
		if walker.Stat().IsDir() {
			continue // Skip directories
		}

		relPath, err := filepath.Rel(c.remoteDir, path)
		if err != nil {
			continue // Skip invalid paths
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".meta":
			// Check if corresponding .data file exists
			dataPath := strings.TrimSuffix(path, ".meta") + ".data"
			_, err := c.sftpClient.Stat(dataPath)
			if err != nil {
				issues = append(issues, fmt.Sprintf("orphaned metadata file: %s (no corresponding .data file)", relPath))
				continue
			}

			// Check if metadata is valid
			metadataFile, err := c.sftpClient.Open(path)
			if err != nil {
				issues = append(issues, fmt.Sprintf("failed to open metadata file: %s: %v", relPath, err))
				continue
			}

			var metadata Metadata
			err = json.NewDecoder(metadataFile).Decode(&metadata)
			metadataFile.Close()
			if err != nil {
				issues = append(issues, fmt.Sprintf("corrupted metadata file: %s: %v", relPath, err))
				continue
			}

			// If hash is present, verify it
			if metadata.Hash != "" {
				dataFile, err := c.sftpClient.Open(dataPath)
				if err != nil {
					issues = append(issues, fmt.Sprintf("failed to open data file for hash verification: %s: %v", strings.TrimSuffix(relPath, ".meta")+".data", err))
					continue
				}

				hash := sha256.New()
				if _, err := io.Copy(hash, dataFile); err != nil {
					dataFile.Close()
					issues = append(issues, fmt.Sprintf("failed to read data file for hash verification: %s: %v", strings.TrimSuffix(relPath, ".meta")+".data", err))
					continue
				}
				dataFile.Close()

				computedHash := hex.EncodeToString(hash.Sum(nil))
				if computedHash != metadata.Hash {
					issues = append(issues, fmt.Sprintf("hash mismatch for %s: stored=%s computed=%s", strings.TrimSuffix(relPath, ".meta"), metadata.Hash, computedHash))
				}
			}

		case ".data":
			// Check if corresponding .meta file exists
			metaPath := strings.TrimSuffix(path, ".data") + ".meta"
			_, err := c.sftpClient.Stat(metaPath)
			if err != nil {
				issues = append(issues, fmt.Sprintf("orphaned data file: %s (no corresponding .meta file)", relPath))
			}
		}
	}

	return issues, nil
}
