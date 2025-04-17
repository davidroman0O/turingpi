package ubuntu

import (
	"fmt"
	"log"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"      // Import base tpi types
	"github.com/davidroman0O/turingpi/pkg/tpi/node" // For SSH/SFTP node interactions
)

// ubuntuRuntimeImpl implements the tpi.UbuntuRuntime interface.
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
func newUbuntuRuntime(ip, user, password string) *ubuntuRuntimeImpl {
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

func (ur *ubuntuRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	log.Printf("[UbuntuRuntime@%s] Running command: %s", ur.nodeIP, command)
	stdout, stderr, err = ur.adapter.ExecuteCommand(command)
	if err != nil {
		err = fmt.Errorf("remote command failed: %w", err)
	}
	return
}

func (ur *ubuntuRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	log.Printf("[UbuntuRuntime@%s] Copying file (toRemote: %t): local=%s, remote=%s", ur.nodeIP, toRemote, localPath, remotePath)
	err := ur.adapter.CopyFile(localPath, remotePath, toRemote)
	if err != nil {
		err = fmt.Errorf("file copy failed: %w", err)
	}
	return err
}
