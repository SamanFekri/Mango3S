package update

import "sync"

// State is where the update flow currently stands.
type State string

// States, in the order a successful update passes through them.
const (
	StateIdle            State = "idle"
	StateChecking        State = "checking"
	StateUpToDate        State = "up_to_date"
	StateUpdateAvailable State = "update_available"
	StateDownloading     State = "downloading"
	StateReadyToInstall  State = "ready_to_install"
	StateInstalling      State = "installing"
	StateError           State = "error"
)

// Status is a snapshot sent to the frontend as-is (JSON tags matter).
type Status struct {
	Connected      bool   `json:"connected"` // an activation key has been accepted
	Revoked        bool   `json:"revoked"`   // the platform returned 403; stop checking automatically
	State          State  `json:"state"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	ReleaseNotes   string `json:"releaseNotes,omitempty"`
	// DownloadDone/DownloadTotal are reserved; the actual download progress
	// while State is StateDownloading is reported through the existing Jobs
	// panel (kind "update"), not these fields.
	DownloadDone  int64  `json:"downloadDone,omitempty"`
	DownloadTotal int64  `json:"downloadTotal,omitempty"`
	Error         string `json:"error,omitempty"`
	LastCheckedAt string `json:"lastCheckedAt,omitempty"`

	// artifactPath/artifactFormat are where the verified download landed and
	// what Apply needs to know to install it. Not sent to the frontend.
	artifactPath   string
	artifactFormat string
}

// Tracker holds the current Status, safe for concurrent use by the
// background check/heartbeat goroutine and bound methods called from the UI.
type Tracker struct {
	mu     sync.RWMutex
	status Status
}

// NewTracker returns a tracker starting in StateIdle.
func NewTracker(currentVersion string) *Tracker {
	return &Tracker{status: Status{State: StateIdle, CurrentVersion: currentVersion}}
}

// Get returns the current status.
func (t *Tracker) Get() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// PendingArtifact returns the verified download waiting to be applied, if any
// (set by SetPendingArtifact, cleared by ClearPendingArtifact).
func (t *Tracker) PendingArtifact() (path, format string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status.artifactPath, t.status.artifactFormat
}

// SetPendingArtifact records a verified, not-yet-applied download.
func (t *Tracker) SetPendingArtifact(path, format string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.artifactPath, t.status.artifactFormat = path, format
}

// ClearPendingArtifact forgets the pending download (after Apply runs, or on error).
func (t *Tracker) ClearPendingArtifact() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.artifactPath, t.status.artifactFormat = "", ""
}

// Update applies fn under the lock and returns the resulting status.
func (t *Tracker) Update(fn func(*Status)) Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(&t.status)
	return t.status
}
