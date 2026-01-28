package storage

import "time"

// ChangeEvent represents a Kubernetes resource change
type ChangeEvent struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Namespace   string    `json:"namespace"`
	Kind        string    `json:"kind"` // Deployment, ConfigMap, Secret
	Name        string    `json:"name"`
	Action      string    `json:"action"`   // ADDED, MODIFIED, DELETED
	Diff        string    `json:"diff"`     // JSON diff or text diff
	Metadata    string    `json:"metadata"` // JSON metadata (labels, annotations, etc)
	ImageBefore string    `json:"image_before,omitempty"`
	ImageAfter  string    `json:"image_after,omitempty"`
}

// Stats represents dashboard statistics
type Stats struct {
	TotalChanges    int64            `json:"total_changes"`
	ChangesLast24h  int64            `json:"changes_last_24h"`
	ChangesPerHour  float64          `json:"changes_per_hour"`
	TopModifiedApps []AppChangeCount `json:"top_modified_apps"`
	RecentImages    []string         `json:"recent_images"`
	ChangesByKind   map[string]int64 `json:"changes_by_kind"`
	ChangesByAction map[string]int64 `json:"changes_by_action"`
}

// AppChangeCount represents changes per app
type AppChangeCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// Filter represents query filters
type Filter struct {
	Namespace string
	Kind      string
	Name      string
	Action    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
	Offset    int
}
