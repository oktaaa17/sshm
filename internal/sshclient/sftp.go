package sshclient

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// UploadFile copies a local file to a remote path over SFTP.
func UploadFile(client *ssh.Client, localPath, remotePath string) (int64, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("failed to start SFTP subsystem: %w", err)
	}
	defer sc.Close()

	src, err := os.Open(filepath.Clean(localPath)) // #nosec G304 -- local source path provided explicitly by CLI user
	if err != nil {
		return 0, err
	}
	defer src.Close()

	// If remotePath ends with / or is an existing remote directory, append local basename.
	if strings.HasSuffix(remotePath, "/") {
		remotePath = path.Join(remotePath, filepath.Base(localPath))
	} else if info, err := sc.Stat(remotePath); err == nil && info.IsDir() {
		remotePath = path.Join(remotePath, filepath.Base(localPath))
	}

	if dir := path.Dir(remotePath); dir != "" && dir != "." {
		_ = sc.MkdirAll(dir)
	}

	dst, err := sc.Create(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	return n, err
}

// DownloadFile copies a remote file to a local path over SFTP.
func DownloadFile(client *ssh.Client, remotePath, localPath string) (int64, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("failed to start SFTP subsystem: %w", err)
	}
	defer sc.Close()

	src, err := sc.Open(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer src.Close()

	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		localPath = filepath.Join(localPath, path.Base(remotePath))
	} else if strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, `\`) {
		if err := os.MkdirAll(localPath, 0o750); err != nil {
			return 0, err
		}
		localPath = filepath.Join(localPath, path.Base(remotePath))
	}

	dst, err := os.Create(filepath.Clean(localPath)) // #nosec G304 -- local destination path provided explicitly by CLI user
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	return n, err
}
