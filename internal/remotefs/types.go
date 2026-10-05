package remotefs

import (
	"time"
)

// SSHConfig defines the parameters needed to establish an SSH/SFTP connection.
type SSHConfig struct {
	ID            string `json:"id"`
	Label         string `json:"label,omitempty"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	AuthType      string `json:"authType"` // "password", "key_file", "agent"
	Password      string `json:"password,omitempty"`
	KeyPath       string `json:"keyPath,omitempty"`
	KeyPassphrase string `json:"keyPassphrase,omitempty"`
	RemotePath    string `json:"remotePath"` // Remote root folder, e.g. "/var/www/myproject"
}

// ConnectionStatus reports the state of an active SSH/SFTP session.
type ConnectionStatus struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	User        string    `json:"user"`
	RemotePath  string    `json:"remotePath"`
	Connected   bool      `json:"connected"`
	ConnectedAt time.Time `json:"connectedAt"`
	Error       string    `json:"error,omitempty"`
}
