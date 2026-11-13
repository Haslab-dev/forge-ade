package board

// Task represents a workflow board task item.
type Task struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"` // e.g. ADE-1
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Status        string   `json:"status"`   // todo, in_progress, in_review, in_testing, done
	Priority      string   `json:"priority"` // low, medium, high, urgent
	Assignee      string   `json:"assignee,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	SessionID     string   `json:"sessionId,omitempty"`
	TaskKind      string   `json:"taskKind,omitempty"`
	WorkspacePath string   `json:"workspacePath,omitempty"`
	CreatedAt     int64    `json:"createdAt"`
	UpdatedAt     int64    `json:"updatedAt"`
	CompletedAt   int64    `json:"completedAt,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	FilesTouched  int      `json:"filesTouched,omitempty"`
}
