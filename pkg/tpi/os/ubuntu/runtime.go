package ubuntu

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
	"github.com/davidroman0O/turingpi/pkg/tpi/node"
)

// localRuntimeImpl implements the LocalRuntime interface.
type localRuntimeImpl struct {
	cacheDir string
}

// Ensure implementation satisfies the interface at compile time
var _ tpi.LocalRuntime = (*localRuntimeImpl)(nil)

// newLocalRuntime creates an instance of the local runtime helper.
func newLocalRuntime(cacheDir string) tpi.LocalRuntime {
	return &localRuntimeImpl{
		cacheDir: cacheDir,
	}
}

func (lr *localRuntimeImpl) ReadFile(localPath string) ([]byte, error) {
	log.Printf("[LocalRuntime] Reading local file: %s", localPath)
	return os.ReadFile(localPath)
}

func (lr *localRuntimeImpl) WriteFile(localPath string, data []byte, perm os.FileMode) error {
	log.Printf("[LocalRuntime] Writing %d bytes to local file: %s (Mode: %o)", len(data), localPath, perm)
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for file: %w", err)
	}
	return os.WriteFile(localPath, data, perm)
}

func (lr *localRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	log.Printf("[LocalRuntime] Running local command: %s", command)
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		err = fmt.Errorf("local command failed: %w. Stderr: %s", err, stderr)
	}
	return
}

func (lr *localRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	// This is a stub for local-to-local copy
	if !toRemote {
		// Local to local copy
		srcFile, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer srcFile.Close()

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(remotePath), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		dstFile, err := os.Create(remotePath)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	}

	return fmt.Errorf("localRuntime.CopyFile can't copy to remote - use a remote runtime instead")
}

// ubuntuRuntimeImpl implements the UbuntuRuntime interface.
type ubuntuRuntimeImpl struct {
	nodeIP   string
	user     string
	password string
	adapter  node.NodeAdapter
}

// Ensure implementation satisfies the interface at compile time
var _ tpi.UbuntuRuntime = (*ubuntuRuntimeImpl)(nil)

// newUbuntuRuntime creates an instance of the runtime helper.
// Note: This is internal to the ubuntu package usually, called by the post-installer.
func newUbuntuRuntime(ip, user, password string) tpi.UbuntuRuntime {
	runtime := &ubuntuRuntimeImpl{
		nodeIP:   ip,
		user:     user,
		password: password,
	}

	// Create the node adapter
	runtime.adapter = node.NewNodeAdapter(node.SSHConfig{
		Host:     ip,
		User:     user,
		Password: password,
		Timeout:  10 * time.Second,
	})

	return runtime
}

// RunCommand executes a command on the remote Ubuntu system
func (ur *ubuntuRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	log.Printf("[UbuntuRuntime@%s] Running command: %s", ur.nodeIP, command)
	stdout, stderr, err = ur.adapter.ExecuteCommand(command)
	if err != nil {
		err = fmt.Errorf("remote command failed: %w", err)
	}
	return
}

// CopyFile transfers a file between the local and remote system
func (ur *ubuntuRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	log.Printf("[UbuntuRuntime@%s] Copying file (toRemote: %t): local=%s, remote=%s", ur.nodeIP, toRemote, localPath, remotePath)

	fileCache := ur.adapter.FileOperations()
	if fileCache == nil {
		return fmt.Errorf("file operations not available")
	}

	ctx := context.Background()

	if toRemote {
		file, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("failed to open local file: %w", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat local file: %w", err)
		}

		metadata := cache.Metadata{
			Filename: filepath.Base(remotePath),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		}

		if _, err := fileCache.Put(ctx, remotePath, metadata, file); err != nil {
			return fmt.Errorf("file copy to remote failed: %w", err)
		}
	} else {
		file, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create local file: %w", err)
		}
		defer file.Close()

		_, reader, err := fileCache.Get(ctx, remotePath, true)
		if err != nil {
			return fmt.Errorf("file copy from remote failed: %w", err)
		}
		if reader == nil {
			return fmt.Errorf("no content received from remote")
		}
		defer reader.Close()

		if _, err := io.Copy(file, reader); err != nil {
			return fmt.Errorf("failed to write local file: %w", err)
		}
	}
	return nil
}
