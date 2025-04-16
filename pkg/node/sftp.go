package node

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Helper to establish SSH client connection (could be shared)
func getNodeSSHClient(ip, user, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second, // Connection timeout
	}
	addr := fmt.Sprintf("%s:22", ip)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	return client, nil
}

// CopyFile transfers a file between the local machine and the remote node using SFTP.
func CopyFile(ip, user, password, localPath, remotePath string, toRemote bool) error {
	log.Printf("[NODE SFTP] Attempting file copy. ToRemote: %t, Local: %s, Remote: %s", toRemote, localPath, remotePath)

	sshClient, err := getNodeSSHClient(ip, user, password)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection for SFTP: %w", err)
	}
	defer sshClient.Close()

	log.Println("[NODE SFTP] Creating SFTP client...")
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}
	defer sftpClient.Close()

	if toRemote {
		// Local to Remote
		log.Printf("[NODE SFTP] Uploading %s to %s...", localPath, remotePath)
		remoteDir := filepath.Dir(remotePath)
		// Ensure remote directory exists
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			if _, statErr := sftpClient.Stat(remoteDir); os.IsNotExist(statErr) {
				return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
			}
			log.Printf("[NODE SFTP] Remote directory %s likely already exists.", remoteDir)
		} else {
			log.Printf("[NODE SFTP] Created remote directory %s.", remoteDir)
		}

		srcFile, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %w", localPath, err)
		}
		defer srcFile.Close()

		dstFile, err := sftpClient.Create(remotePath)
		if err != nil {
			return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
		}
		defer dstFile.Close()

		bytesCopied, err := io.Copy(dstFile, srcFile)
		if err != nil {
			_ = sftpClient.Remove(remotePath) // Attempt cleanup
			return fmt.Errorf("failed to copy content to remote: %w", err)
		}
		log.Printf("[NODE SFTP] Successfully uploaded %d bytes.", bytesCopied)

	} else {
		// Remote to Local
		log.Printf("[NODE SFTP] Downloading %s to %s...", remotePath, localPath)
		localDir := filepath.Dir(localPath)
		// Ensure local directory exists
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
		}

		srcFile, err := sftpClient.Open(remotePath)
		if err != nil {
			return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create local file %s: %w", localPath, err)
		}
		defer dstFile.Close()

		bytesCopied, err := io.Copy(dstFile, srcFile)
		if err != nil {
			_ = os.Remove(localPath) // Attempt cleanup
			return fmt.Errorf("failed to copy content to local: %w", err)
		}
		log.Printf("[NODE SFTP] Successfully downloaded %d bytes.", bytesCopied)
	}

	return nil
}

// TODO: Implement CopyDirectory for recursive copying
