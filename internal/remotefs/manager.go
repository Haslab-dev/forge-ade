package remotefs

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/explorer"
)

// Manager coordinates all remote SSH/SFTP sessions and routes filesystem operations.
type Manager struct {
	mu           sync.RWMutex
	clients      map[string]*Client
	activeConnID string
}

// NewManager creates an instance of remotefs Manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
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

	m.mu.Lock()
	if old, exists := m.clients[cfg.ID]; exists {
		_ = old.Close()
		delete(m.clients, cfg.ID)
	}
	m.mu.Unlock()

	client, err := Dial(cfg)
	if err != nil {
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
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return nil
	}

	delete(m.clients, id)
	if m.activeConnID == id {
		m.activeConnID = ""
	}
	return client.Close()
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

// CopyPath routes file copying to remote SFTP.
func (m *Manager) CopyPath(src, dst string) (bool, error) {
	cSrc, srcRemote, isSrcRemote := m.ParseRemotePath(src)
	_, dstRemote, isDstRemote := m.ParseRemotePath(dst)
	if !isSrcRemote || !isDstRemote || cSrc == nil {
		return false, nil
	}
	return true, cSrc.CopyPath(srcRemote, dstRemote)
}
