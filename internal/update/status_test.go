package update

import "testing"

func TestTrackerUpdateAndPendingArtifact(t *testing.T) {
	tr := NewTracker("1.0.0")
	if got := tr.Get().CurrentVersion; got != "1.0.0" {
		t.Fatalf("CurrentVersion = %q, want 1.0.0", got)
	}

	tr.Update(func(s *Status) { s.State = StateUpdateAvailable; s.LatestVersion = "1.1.0" })
	if got := tr.Get(); got.State != StateUpdateAvailable || got.LatestVersion != "1.1.0" {
		t.Fatalf("unexpected status after update: %+v", got)
	}

	tr.SetPendingArtifact("/tmp/x.zip", "zip")
	path, format := tr.PendingArtifact()
	if path != "/tmp/x.zip" || format != "zip" {
		t.Fatalf("PendingArtifact = (%q, %q)", path, format)
	}

	tr.ClearPendingArtifact()
	path, format = tr.PendingArtifact()
	if path != "" || format != "" {
		t.Fatalf("expected cleared artifact, got (%q, %q)", path, format)
	}
}

func TestNewDeviceFingerprintIsUniqueAndHex(t *testing.T) {
	a := NewDeviceFingerprint()
	b := NewDeviceFingerprint()
	if a == b {
		t.Fatalf("two fingerprints collided: %q", a)
	}
	if len(a) != 32 {
		t.Fatalf("fingerprint length = %d, want 32 (16 bytes hex-encoded)", len(a))
	}
}
