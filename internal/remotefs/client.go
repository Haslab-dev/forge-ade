package remotefs

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client wraps an SSH and SFTP connection to a remote machine.
type Client struct {
	config     SSHConfig
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	mu         sync.RWMutex
	createdAt  time.Time
	closed     bool

	askpassMu   sync.Mutex
	askpassPath string
}

// Dial creates a new SSH and SFTP client from the provided configuration.
func Dial(cfg SSHConfig) (*Client, error) {
	if cfg.Port <= 0 {
		cfg.Port = 22
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("ssh username is required")
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("ssh host is required")
	}

	authMethods := []ssh.AuthMethod{}

	switch cfg.AuthType {
	case "password":
		if cfg.Password == "" {
			return nil, fmt.Errorf("password is required for password authentication")
		}
		authMethods = append(authMethods, ssh.Password(cfg.Password))

	case "key_file", "key":
		keyPath := cfg.KeyPath
		if keyPath == "" {
			// Try default keys if not specified
			home, _ := os.UserHomeDir()
			for _, defKey := range []string{"id_ed25519", "id_rsa"} {
				candidate := filepath.Join(home, ".ssh", defKey)
				if _, err := os.Stat(candidate); err == nil {
					keyPath = candidate
					break
				}
			}
		} else {
			keyPath = expandHomeDir(keyPath)
		}

		if keyPath == "" {
			return nil, fmt.Errorf("no ssh private key found")
		}

		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
		}

		var signer ssh.Signer
		if cfg.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	case "agent":
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			conn, err := net.Dial("unix", sock)
			if err == nil {
				ag := agent.NewClient(conn)
				authMethods = append(authMethods, ssh.PublicKeysCallback(ag.Signers))
			}
		}
		if len(authMethods) == 0 {
			return nil, fmt.Errorf("ssh-agent not available or no keys found")
		}

	default:
		// Try key auth first, then password if provided
		if cfg.KeyPath != "" {
			keyPath := expandHomeDir(cfg.KeyPath)
			if keyBytes, err := os.ReadFile(keyPath); err == nil {
				var signer ssh.Signer
				if cfg.KeyPassphrase != "" {
					signer, _ = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.KeyPassphrase))
				} else {
					signer, _ = ssh.ParsePrivateKey(keyBytes)
				}
				if signer != nil {
					authMethods = append(authMethods, ssh.PublicKeys(signer))
				}
			}
		}
		if cfg.Password != "" {
			authMethods = append(authMethods, ssh.Password(cfg.Password))
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid authentication method configured")
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For ADE local workstation dev usage
		Timeout:         12 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	sshConn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, fmt.Errorf("sftp init on %s: %w", addr, err)
	}

	// Verify remote path or resolve default
	remotePath := cfg.RemotePath
	if remotePath == "" || remotePath == "~" {
		wd, err := sftpClient.Getwd()
		if err == nil && wd != "" {
			remotePath = wd
		} else {
			remotePath = "/"
		}
	}
	cfg.RemotePath = path.Clean(remotePath)

	return &Client{
		config:     cfg,
		sshClient:  sshConn,
		sftpClient: sftpClient,
		createdAt:  time.Now(),
	}, nil
}

// Config returns the SSH configuration for this client.
func (c *Client) Config() SSHConfig {
	return c.config
}

// Close closes both SFTP and SSH connections.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.askpassPath != "" {
		_ = os.Remove(c.askpassPath)
		c.askpassPath = ""
	}
	var errs []string
	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("error closing client: %s", strings.Join(errs, "; "))
	}
	return nil
}

// shellQuote single-quotes a path for safe use inside a remote shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// SSHCommand builds the argv of a LOCAL ssh process that, when run under a
// PTY, drops the user into an interactive login shell on the remote host at
// remotePath. For password (or passphrase-protected key) connections it also
// returns SSH_ASKPASS env vars wired to a per-connection helper script, so the
// spawned ssh reuses this connection's credentials instead of prompting the
// user again — the SFTP session authenticated already, the shell must not ask
// for the same password a second time. The helper is removed on Close.
func (c *Client) SSHCommand(remotePath string) (string, []string, []string) {
	cfg := c.Config()
	args := []string{
		"-p", strconv.Itoa(cfg.Port),
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if cfg.AuthType == "key_file" && cfg.KeyPath != "" {
		args = append(args, "-i", cfg.KeyPath)
	}
	args = append(args, cfg.User+"@"+cfg.Host, "-t",
		"cd "+shellQuote(remotePath)+" && exec $SHELL -l")

	var env []string
	if secret := c.promptSecret(); secret != "" {
		if script, err := c.ensureAskpass(secret); err == nil {
			env = append(env,
				"SSH_ASKPASS="+script,
				"SSH_ASKPASS_REQUIRE=force",
				"DISPLAY=forge-ade-askpass",
			)
		}
	}
	return "ssh", args, env
}

// promptSecret returns the credential a spawned ssh process would interactively
// prompt for: the login password for password auth, or the key passphrase for
// passphrase-protected keys. Empty for key/agent auth (no prompt expected).
func (c *Client) promptSecret() string {
	cfg := c.Config()
	switch cfg.AuthType {
	case "password":
		return cfg.Password
	case "key_file", "key":
		return cfg.KeyPassphrase
	default:
		if cfg.Password != "" {
			return cfg.Password
		}
		return ""
	}
}

// ensureAskpass writes a tiny helper script that prints secret, so ssh's
// SSH_ASKPASS machinery can answer its own password prompt non-interactively.
// The file lives in a user-only temp dir with 0600 permissions and is deleted
// when the connection closes.
func (c *Client) ensureAskpass(secret string) (string, error) {
	c.askpassMu.Lock()
	defer c.askpassMu.Unlock()
	if c.askpassPath != "" {
		return c.askpassPath, nil
	}
	dir, err := os.MkdirTemp("", "forge-ssh-askpass-")
	if err != nil {
		return "", err
	}
	_ = os.Chmod(dir, 0o700)
	script := filepath.Join(dir, "askpass.sh")
	body := "#!/bin/sh\necho '" + strings.ReplaceAll(secret, "'", `'\''`) + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := os.Chmod(script, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	c.askpassPath = script
	return script, nil
}

// ListDirectory lists entries in a remote directory at depth 1.
func (c *Client) ListDirectory(dirPath string) ([]*explorer.FileInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.sftpClient == nil {
		return nil, fmt.Errorf("sftp client is closed")
	}

	cleanDir := path.Clean(dirPath)
	entries, err := c.sftpClient.ReadDir(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("sftp readdir %s: %w", cleanDir, err)
	}

	var items []*explorer.FileInfo
	for _, entry := range entries {
		name := entry.Name()
		fullPath := path.Join(cleanDir, name)
		isDir := entry.IsDir()

		items = append(items, &explorer.FileInfo{
			Name:       name,
			Path:       fullPath,
			IsDir:      isDir,
			Size:       entry.Size(),
			Mode:       entry.Mode().String(),
			ModTime:    entry.ModTime().Format("2006-01-02 15:04:05"),
			Hidden:     strings.HasPrefix(name, "."),
			GitIgnored: false,
		})
	}

	// Directories first, then alphabetical
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

// ReadFile reads the full contents of a remote file.
func (c *Client) ReadFile(filePath string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.sftpClient == nil {
		return nil, fmt.Errorf("sftp client is closed")
	}

	cleanPath := path.Clean(filePath)
	f, err := c.sftpClient.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", cleanPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("sftp read %s: %w", cleanPath, err)
	}
	return data, nil
}

// WriteFile writes data to a remote file, creating parent directories if needed.
func (c *Client) WriteFile(filePath string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sftpClient == nil {
		return fmt.Errorf("sftp client is closed")
	}

	cleanPath := path.Clean(filePath)
	parent := path.Dir(cleanPath)
	if err := c.sftpClient.MkdirAll(parent); err != nil {
		return fmt.Errorf("sftp mkdirall %s: %w", parent, err)
	}

	f, err := c.sftpClient.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", cleanPath, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("sftp write %s: %w", cleanPath, err)
	}
	return nil
}

// CreateFile creates an empty remote file.
func (c *Client) CreateFile(filePath string) error {
	return c.WriteFile(filePath, []byte{})
}

// CreateFolder creates a remote directory and any missing parents.
func (c *Client) CreateFolder(folderPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sftpClient == nil {
		return fmt.Errorf("sftp client is closed")
	}

	cleanPath := path.Clean(folderPath)
	if err := c.sftpClient.MkdirAll(cleanPath); err != nil {
		return fmt.Errorf("sftp mkdirall %s: %w", cleanPath, err)
	}
	return nil
}

// DeleteFile removes a remote file or recursively deletes a directory.
func (c *Client) DeleteFile(targetPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sftpClient == nil {
		return fmt.Errorf("sftp client is closed")
	}

	cleanPath := path.Clean(targetPath)
	stat, err := c.sftpClient.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("sftp stat %s: %w", cleanPath, err)
	}

	if stat.IsDir() {
		return c.removeDirRecursive(cleanPath)
	}
	return c.sftpClient.Remove(cleanPath)
}

func (c *Client) removeDirRecursive(dirPath string) error {
	entries, err := c.sftpClient.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sub := path.Join(dirPath, entry.Name())
		if entry.IsDir() {
			if err := c.removeDirRecursive(sub); err != nil {
				return err
			}
		} else {
			if err := c.sftpClient.Remove(sub); err != nil {
				return err
			}
		}
	}
	return c.sftpClient.RemoveDirectory(dirPath)
}

// RenameFile renames or moves a remote file or folder.
func (c *Client) RenameFile(oldPath, newPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sftpClient == nil {
		return fmt.Errorf("sftp client is closed")
	}

	cleanOld := path.Clean(oldPath)
	cleanNew := path.Clean(newPath)
	parent := path.Dir(cleanNew)
	_ = c.sftpClient.MkdirAll(parent)

	if err := c.sftpClient.Rename(cleanOld, cleanNew); err != nil {
		return fmt.Errorf("sftp rename %s -> %s: %w", cleanOld, cleanNew, err)
	}
	return nil
}

// CopyPath copies a file or folder recursively on the remote system.
func (c *Client) CopyPath(src, dst string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.sftpClient == nil {
		return fmt.Errorf("sftp client is closed")
	}

	cleanSrc := path.Clean(src)
	cleanDst := path.Clean(dst)

	stat, err := c.sftpClient.Stat(cleanSrc)
	if err != nil {
		return fmt.Errorf("sftp stat src %s: %w", cleanSrc, err)
	}

	if stat.IsDir() {
		return c.copyDirRecursive(cleanSrc, cleanDst)
	}
	return c.copyFileSingle(cleanSrc, cleanDst)
}

func (c *Client) copyFileSingle(src, dst string) error {
	_ = c.sftpClient.MkdirAll(path.Dir(dst))

	srcFile, err := c.sftpClient.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := c.sftpClient.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (c *Client) copyDirRecursive(srcDir, dstDir string) error {
	if err := c.sftpClient.MkdirAll(dstDir); err != nil {
		return err
	}
	entries, err := c.sftpClient.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		subSrc := path.Join(srcDir, entry.Name())
		subDst := path.Join(dstDir, entry.Name())
		if entry.IsDir() {
			if err := c.copyDirRecursive(subSrc, subDst); err != nil {
				return err
			}
		} else {
			if err := c.copyFileSingle(subSrc, subDst); err != nil {
				return err
			}
		}
	}
	return nil
}

func expandHomeDir(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
