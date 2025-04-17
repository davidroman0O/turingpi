package node

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
)

// CopyFile implements NodeAdapter
func (a *nodeAdapter) CopyFile(localPath, remotePath string, toRemote bool) error {
	if err := a.checkClosed(); err != nil {
		return err
	}

	log.Printf("[NODE SFTP] Attempting file copy. ToRemote: %t, Local: %s, Remote: %s", toRemote, localPath, remotePath)

	sshClient, err := a.getSSHClient()
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection for SFTP: %w", err)
	}

	log.Println("[NODE SFTP] Creating SFTP client...")
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}

	// Track the SFTP client
	a.mu.Lock()
	a.sftp[sftpClient] = true
	a.mu.Unlock()

	defer func() {
		sftpClient.Close()
		a.mu.Lock()
		delete(a.sftp, sftpClient)
		a.mu.Unlock()
	}()

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
