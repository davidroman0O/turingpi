package ubuntu

import (
	"fmt"
	"log"
	"time"

	"github.com/davidroman0O/turingpi/pkg/node" // For SSH/SFTP node interactions
	"github.com/davidroman0O/turingpi/pkg/tpi"  // Import base tpi types
)

// ubuntuRuntimeImpl implements the tpi.UbuntuRuntime interface.
type ubuntuRuntimeImpl struct {
	nodeIP   string
	user     string
	password string
	// Could hold an established SSH client later for efficiency
}

// Ensure implementation satisfies the interface at compile time
var _ tpi.UbuntuRuntime = (*ubuntuRuntimeImpl)(nil)

// newUbuntuRuntime creates an instance of the runtime helper.
// Note: This is internal to the ubuntu package usually, called by the post-installer.
func newUbuntuRuntime(ip, user, password string) *ubuntuRuntimeImpl {
	return &ubuntuRuntimeImpl{
		nodeIP:   ip,
		user:     user,
		password: password,
	}
}

func (ur *ubuntuRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	log.Printf("[UbuntuRuntime@%s] Running command: %s", ur.nodeIP, command)
	// TODO: Add timeout handling to pkg/node or here
	stdout, stderr, err = node.ExecuteCommand(ur.nodeIP, ur.user, ur.password, command)
	if err != nil {
		err = fmt.Errorf("remote command failed: %w", err)
	}
	// Output is already logged by node.ExecuteCommand
	return
}

func (ur *ubuntuRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	log.Printf("[UbuntuRuntime@%s] Copying file (toRemote: %t): local=%s, remote=%s", ur.nodeIP, toRemote, localPath, remotePath)
	err := node.CopyFile(ur.nodeIP, ur.user, ur.password, localPath, remotePath, toRemote)
	if err != nil {
		err = fmt.Errorf("file copy failed: %w", err)
	}
	return err
}
