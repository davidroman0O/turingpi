package tpi

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// --- LocalRuntime Implementation ---

// localRuntimeImpl implements the LocalRuntime interface.
type localRuntimeImpl struct {
	// No fields needed for now, could hold context/logger later
}

func (lr *localRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	// This seems redundant if called from the post-install context where
	// we always interact via the remote runtime. Let's assume this is meant
	// for local-to-local or requires node IP/creds if used for remote.
	// For now, return error indicating it needs clarification or a remote runtime.
	return fmt.Errorf("localRuntime.CopyFile needs clarification - use remoteRuntime for node operations")
}

func (lr *localRuntimeImpl) ReadFile(localPath string) ([]byte, error) {
	log.Printf("[LocalRuntime] Reading local file: %s", localPath)
	return os.ReadFile(localPath)
}

func (lr *localRuntimeImpl) WriteFile(localPath string, data []byte, perm os.FileMode) error {
	log.Printf("[LocalRuntime] Writing %d bytes to local file: %s (Mode: %o)", len(data), localPath, perm)
	return os.WriteFile(localPath, data, perm)
}

func (lr *localRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	log.Printf("[LocalRuntime] Running local command: %s", command)
	// TODO: Implement timeout using context
	cmd := exec.Command("bash", "-c", command)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	if err != nil {
		err = fmt.Errorf("local command failed: %w. Stderr: %s", err, stderr)
	}
	log.Printf("[LocalRuntime] stdout: %s", stdout)
	log.Printf("[LocalRuntime] stderr: %s", stderr)
	return
}

// NewLocalRuntimeImpl creates a new instance of the LocalRuntime implementation.
func NewLocalRuntimeImpl() *localRuntimeImpl {
	return &localRuntimeImpl{}
}
