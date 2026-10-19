package update

import (
	"crypto/rand"
	"encoding/hex"
)

// NewDeviceFingerprint returns a random, stable-once-generated identifier for
// this installation (persisted in config.Settings.DeviceFingerprint). It only
// needs to be unique per client key, not globally.
func NewDeviceFingerprint() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
