package remotefs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/explorer"
)

// Manager coordinates all remote SSH/SFTP sessions and routes filesystem operations.
type Manager struct {
	mu           sync.RWMutex
	clients      map[string]*Client
	activeConnID string
	bus          *events.Bus
}

// NewManager creates an instance of remotefs Manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// SetEventBus sets the events.Bus for broadcasting remote connection updates.
func (m *Manager) SetEventBus(bus *events.Bus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bus = bus
}

func (m *Manager) emitEvent(typ events.EventType, data map[string]interface{}) {
	m.mu.RLock()
	bus := m.bus
	m.mu.RUnlock()
	if bus != nil {
		bus.Publish(events.Event{
			Type: typ,
			Data: data,
		})
	}
}

// Connect establishes a new SSH/SFTP session and registers it.
func (m *Manager) Connect(cfg SSHConfig) (*ConnectionStatus, error) {
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("ssh-%s-%d-%s", cfg.Host, cfg.Port, uuid.New().String()[:6])
	}
	if cfg.Label == "" {
		cfg.Label = fmt.Sprintf("%s@%s:%s", cfg.User, cfg.Host, cfg.RemotePath)
	}

	m.emitEvent(events.SSHHandshake, map[string]interface{}{
		"id":     cfg.ID,
		"phase":  "start",
		"host":   cfg.Host,
		"port":   cfg.Port,
		"user":   cfg.User,
		"status": "initiating handshake",
	})

	m.mu.Lock()
	if old, exists := m.clients[cfg.ID]; exists {
		_ = old.Close()
		delete(m.clients, cfg.ID)
	}
	m.mu.Unlock()

	client, err := DialWithProgress(cfg, func(phase, detail string) {
		m.emitEvent(events.SSHHandshake, map[string]interface{}{
			"id":     cfg.ID,
			"phase":  phase,
			"host":   cfg.Host,
			"port":   cfg.Port,
			"user":   cfg.User,
			"status": detail,
		})
	})
	if err != nil {
		m.emitEvent(events.SSHError, map[string]interface{}{
			"id":    cfg.ID,
			"host":  cfg.Host,
			"error": err.Error(),
		})
		m.emitEvent(events.SSHDisconnected, map[string]interface{}{
			"id":     cfg.ID,
			"host":   cfg.Host,
			"reason": err.Error(),
		})
		return &ConnectionStatus{
			ID:          cfg.ID,
			Label:       cfg.Label,
			Host:        cfg.Host,
			Port:        cfg.Port,
			User:        cfg.User,
			RemotePath:  cfg.RemotePath,
			Connected:   false,
			ConnectedAt: time.Now(),
			Error:       err.Error(),
		}, err
	}

	m.mu.Lock()
	m.clients[cfg.ID] = client
	m.activeConnID = cfg.ID
	m.mu.Unlock()

	// Start watcher to detect unexpected server disconnects or network drops
	connID := cfg.ID
	host := cfg.Host
	client.StartWatcher(func(discErr error) {
		m.mu.Lock()
		if m.activeConnID == connID {
			m.activeConnID = ""
		}
		delete(m.clients, connID)
		m.mu.Unlock()

		m.emitEvent(events.SSHDisconnected, map[string]interface{}{
			"id":     connID,
			"host":   host,
			"reason": discErr.Error(),
		})
	})

	m.emitEvent(events.SSHHandshake, map[string]interface{}{
		"id":     cfg.ID,
		"phase":  "completed",
		"host":   cfg.Host,
		"port":   cfg.Port,
		"user":   cfg.User,
		"status": "connected",
	})

	m.emitEvent(events.SSHConnected, map[string]interface{}{
		"id":         cfg.ID,
		"host":       cfg.Host,
		"port":       cfg.Port,
		"user":       cfg.User,
		"remotePath": client.Config().RemotePath,
	})

	status := &ConnectionStatus{
		ID:          cfg.ID,
		Label:       cfg.Label,
		Host:        cfg.Host,
		Port:        cfg.Port,
		User:        cfg.User,
		RemotePath:  client.Config().RemotePath,
		Connected:   true,
		ConnectedAt: client.createdAt,
	}

	return status, nil
}

// Disconnect closes and removes the specified SSH connection.
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	client, exists := m.clients[id]
	if !exists {
		m.mu.Unlock()
		return nil
	}

	delete(m.clients, id)
	if m.activeConnID == id {
		m.activeConnID = ""
	}
	m.mu.Unlock()

	m.emitEvent(events.SSHDisconnected, map[string]interface{}{
		"id":     id,
		"host":   client.Config().Host,
		"reason": "user disconnected",
	})

	return client.Close()
}

// CloseAll closes all active SSH/SFTP clients.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, client := range m.clients {
		_ = client.Close()
		delete(m.clients, id)
	}
	m.activeConnID = ""
}

// GetActiveClient returns the currently active Client or nil.
func (m *Manager) GetActiveClient() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeConnID == "" {
		return nil
	}
	return m.clients[m.activeConnID]
}

// SetActiveConnection sets the active remote session.
func (m *Manager) SetActiveConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeConnID = id
}

// GetClient retrieves a client by ID.
func (m *Manager) GetClient(id string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[id]
}

// ListConnections lists statuses of all registered remote sessions.
func (m *Manager) ListConnections() []ConnectionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []ConnectionStatus
	for id, c := range m.clients {
		cfg := c.Config()
		list = append(list, ConnectionStatus{
			ID:          id,
			Label:       cfg.Label,
			Host:        cfg.Host,
			Port:        cfg.Port,
			User:        cfg.User,
			RemotePath:  cfg.RemotePath,
			Connected:   !c.closed,
			ConnectedAt: c.createdAt,
		})
	}
	return list
}

// ParseRemotePath checks if a path belongs to an SSH session.
// Only explicit ssh://<connId>/<remotePath> paths are remote. Matching by the
// active connection's remote-root prefix was removed deliberately: with a
// remote root of "/" every local absolute path matched, so after using one
// remote workspace every local file/tree call was misrouted to SFTP and the
// explorer/git/files APIs kept failing (or showing the remote) until restart.
func (m *Manager) ParseRemotePath(p string) (client *Client, remotePath string, isRemote bool) {
	if !strings.HasPrefix(p, "ssh://") {
		return nil, p, false
	}
	trimmed := strings.TrimPrefix(p, "ssh://")
	parts := strings.SplitN(trimmed, "/", 2)
	connID := parts[0]
	subPath := "/"
	if len(parts) > 1 {
		subPath = "/" + parts[1]
	}

	m.mu.RLock()
	c, ok := m.clients[connID]
	m.mu.RUnlock()

	if ok && c != nil {
		return c, subPath, true
	}
	// Known connection id but stale/dropped client, or an unknown id: not remote.
	return nil, p, false
}

// ListDirectory routes directory listing to remote SFTP if path is remote.
func (m *Manager) ListDirectory(targetPath string) ([]*explorer.FileInfo, bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return nil, false, nil
	}
	entries, err := c.ListDirectory(remotePath)
	if err != nil {
		return nil, true, err
	}
	// Prefix entry paths with ssh://<connID> so subsequent calls route cleanly
	prefix := fmt.Sprintf("ssh://%s", c.Config().ID)
	for _, entry := range entries {
		entry.Path = fmt.Sprintf("%s%s", prefix, entry.Path)
	}
	return entries, true, nil
}

// ReadFile routes file reading to remote SFTP.
func (m *Manager) ReadFile(targetPath string) ([]byte, bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return nil, false, nil
	}
	data, err := c.ReadFile(remotePath)
	return data, true, err
}

// StatSize routes a remote file-size stat to SFTP. isRemote is false when the
// path is not an ssh:// target or the connection is unknown.
func (m *Manager) StatSize(targetPath string) (int64, bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return -1, false, nil
	}
	size, err := c.StatSize(remotePath)
	return size, true, err
}

// WriteFile routes file saving to remote SFTP.
func (m *Manager) WriteFile(targetPath string, data []byte) (bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return false, nil
	}
	return true, c.WriteFile(remotePath, data)
}

// CreateFile routes file creation to remote SFTP.
func (m *Manager) CreateFile(targetPath string) (bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return false, nil
	}
	return true, c.CreateFile(remotePath)
}

// CreateFolder routes folder creation to remote SFTP.
func (m *Manager) CreateFolder(targetPath string) (bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return false, nil
	}
	return true, c.CreateFolder(remotePath)
}

// DeleteFile routes file/folder deletion to remote SFTP.
func (m *Manager) DeleteFile(targetPath string) (bool, error) {
	c, remotePath, isRemote := m.ParseRemotePath(targetPath)
	if !isRemote || c == nil {
		return false, nil
	}
	return true, c.DeleteFile(remotePath)
}

// RenameFile routes file renaming to remote SFTP.
func (m *Manager) RenameFile(oldPath, newPath string) (bool, error) {
	cOld, oldRemote, isOldRemote := m.ParseRemotePath(oldPath)
	_, newRemote, isNewRemote := m.ParseRemotePath(newPath)
	if !isOldRemote || !isNewRemote || cOld == nil {
		return false, nil
	}
	return true, cOld.RenameFile(oldRemote, newRemote)
}

// CopyPath routes file copying:
// - Remote to Remote: handled via SFTP
// - Local to Remote (Upload): streams local file/dir to remote SFTP
// - Remote to Local (Download): streams remote SFTP file/dir to local filesystem
func (m *Manager) CopyPath(src, dst string) (bool, error) {
	cSrc, srcRemote, isSrcRemote := m.ParseRemotePath(src)
	cDst, dstRemote, isDstRemote := m.ParseRemotePath(dst)

	// Remote to Remote
	if isSrcRemote && isDstRemote {
		if cSrc == nil {
			return true, fmt.Errorf("source remote client not available")
		}
		if cSrc == cDst {
			return true, cSrc.CopyPath(srcRemote, dstRemote)
		}
		return true, fmt.Errorf("cross-server copy not supported")
	}

	// Local to Remote (Upload)
	if !isSrcRemote && isDstRemote {
		if cDst == nil {
			return true, fmt.Errorf("destination remote client not available")
		}
		return true, m.UploadFileToRemote(src, dst)
	}

	// Remote to Local (Download)
	if isSrcRemote && !isDstRemote {
		if cSrc == nil {
			return true, fmt.Errorf("source remote client not available")
		}
		return true, cSrc.DownloadFile(srcRemote, dst)
	}

	return false, nil
}

// UploadFileToRemote uploads a local file or directory to a remote destination (remoteTargetURI can be ssh://... or a remote path).
func (m *Manager) UploadFileToRemote(localPath, remoteTargetURI string) error {
	c, remotePath, isRemote := m.ParseRemotePath(remoteTargetURI)
	if !isRemote || c == nil {
		c = m.GetActiveClient()
		if c == nil {
			return fmt.Errorf("no active remote SSH connection")
		}
		remotePath = remoteTargetURI
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local file stat %s: %w", localPath, err)
	}

	cleanRemote := path.Clean(remotePath)
	// If remote target is an existing directory or ends with a slash, place inside it
	if rStat, err := c.sftpClient.Stat(cleanRemote); err == nil && rStat.IsDir() {
		cleanRemote = path.Join(cleanRemote, filepath.Base(localPath))
	} else if strings.HasSuffix(remoteTargetURI, "/") {
		cleanRemote = path.Join(cleanRemote, filepath.Base(localPath))
	}

	if info.IsDir() {
		return c.UploadDir(localPath, cleanRemote)
	}
	return c.UploadFile(localPath, cleanRemote)
}

// UploadFilesToRemote uploads multiple local files to a remote destination directory URI.
func (m *Manager) UploadFilesToRemote(localPaths []string, remoteDirURI string) ([]string, error) {
	c, remoteDir, isRemote := m.ParseRemotePath(remoteDirURI)
	if !isRemote || c == nil {
		c = m.GetActiveClient()
		if c == nil {
			return nil, fmt.Errorf("no active remote SSH connection")
		}
		remoteDir = remoteDirURI
	}

	cleanDir := path.Clean(remoteDir)
	var uploaded []string
	for _, lp := range localPaths {
		if strings.TrimSpace(lp) == "" {
			continue
		}
		info, err := os.Stat(lp)
		if err != nil {
			return uploaded, fmt.Errorf("file %s not found: %w", lp, err)
		}
		dest := path.Join(cleanDir, filepath.Base(lp))
		if info.IsDir() {
			if err := c.UploadDir(lp, dest); err != nil {
				return uploaded, fmt.Errorf("upload dir %s: %w", lp, err)
			}
		} else {
			if err := c.UploadFile(lp, dest); err != nil {
				return uploaded, fmt.Errorf("upload file %s: %w", lp, err)
			}
		}
		uploaded = append(uploaded, dest)
	}
	return uploaded, nil
}
