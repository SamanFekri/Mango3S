// Package update talks to the web admin platform's desktop-facing /v1/* API
// (see web/docs/API_CONTRACT.md in the companion repo) and applies downloaded
// updates. It has no dependency on package api so it can be tested in
// isolation; internal/api/update.go wires it into the bound App.
package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// HTTPClient is used for every platform API call except artifact downloads
// (replaceable in tests). The timeout is per attempt; see RetryDelays.
var HTTPClient = &http.Client{Timeout: 15 * time.Second}

// DownloadClient fetches update artifacts. It has no overall timeout — a large
// installer on a slow link can legitimately take longer than any fixed limit —
// so a stalled download is bounded by the caller's context (the download job,
// which the user can cancel) and by the response-header timeout below.
var DownloadClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: t}
}()

// RetryDelays are the waits between attempts of a platform API call that
// failed transiently (no response, 429, or 5xx); 4xx answers are final and
// never retried. The platform is optional for the app, so these calls can
// afford to be patient — but they never block anything the user is doing.
var RetryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 9 * time.Second}

// IsTransient reports whether err is worth retrying.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status == http.StatusTooManyRequests || se.Status >= 500
	}
	return true // no HTTP response at all: offline, DNS, refused, timed out
}

// Client talks to one platform deployment.
type Client struct {
	BaseURL string
}

// New returns a client for baseURL (trailing slash optional).
func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/")}
}

// StatusError is returned when the platform responds with a non-2xx status.
// Callers use IsForbidden to detect "this client/installation was disabled".
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("platform returned HTTP %d: %s", e.Status, e.Body)
}

// IsForbidden reports whether err is a 403 — an admin blocked this installation.
func IsForbidden(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == http.StatusForbidden
}

// IsUnauthorized reports whether err is a 401 — the stored installation token
// is no longer valid (the app re-registers to get a new one).
func IsUnauthorized(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == http.StatusUnauthorized
}

// do sends one API call, retrying transient failures per RetryDelays (with
// jitter, so a fleet of installs coming back online doesn't retry in lockstep).
func (c *Client) do(ctx context.Context, method, path, token string, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	for attempt := 0; ; attempt++ {
		err := c.doOnce(ctx, method, path, token, payload, out)
		if err == nil || !IsTransient(err) || attempt >= len(RetryDelays) || ctx.Err() != nil {
			return err
		}
		wait := RetryDelays[attempt]
		wait += time.Duration(rand.Int64N(int64(wait)/4 + 1))
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return err
		}
	}
}

func (c *Client) doOnce(ctx context.Context, method, path, token string, payload []byte, out any) error {
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return &StatusError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// RegisterRequest is sent on first launch (Mango3S is free — no key needed)
// to obtain a long-lived installation token. Re-registering with the same
// fingerprint returns the same installation with a fresh token.
type RegisterRequest struct {
	DeviceFingerprint string `json:"deviceFingerprint"`
	AppVersion        string `json:"appVersion"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	OSVersion         string `json:"osVersion,omitempty"`
	Label             string `json:"label,omitempty"`
}

// RegisterResponse carries the installation token — store it like a password.
type RegisterResponse struct {
	InstallationID    string `json:"installationId"`
	InstallationToken string `json:"installationToken"`
}

// Register registers this installation and returns its token.
func (c *Client) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	var out RegisterResponse
	err := c.do(ctx, http.MethodPost, "/v1/installations/register", "", req, &out)
	return out, err
}

// HeartbeatRequest reports this installation is alive and its current version/OS.
type HeartbeatRequest struct {
	AppVersion string `json:"appVersion"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	OSVersion  string `json:"osVersion,omitempty"`
}

// Heartbeat reports activity for installationID.
func (c *Client) Heartbeat(ctx context.Context, installationID, token string, req HeartbeatRequest) error {
	return c.do(ctx, http.MethodPost, "/v1/installations/"+url.PathEscape(installationID)+"/heartbeat", token, req, nil)
}

// Artifact describes the latest release's downloadable file for this os/arch.
type Artifact struct {
	ID           string `json:"id"`
	FileName     string `json:"fileName"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256"`
	DownloadURL  string `json:"downloadUrl"`
	HasSignature bool   `json:"hasSignature"`
}

// LatestRelease is the "latest" release an update check found, if any.
type LatestRelease struct {
	Version     string   `json:"version"`
	Channel     string   `json:"channel"`
	Notes       string   `json:"notes"`
	PublishedAt string   `json:"publishedAt"`
	Artifact    Artifact `json:"artifact"`
}

// UpdateCheckResult is GET /v1/updates/check's response.
type UpdateCheckResult struct {
	UpdateAvailable bool           `json:"updateAvailable"`
	Latest          *LatestRelease `json:"latest"`
}

// CheckForUpdate asks whether a newer release than currentVersion is published for os/arch/channel.
func (c *Client) CheckForUpdate(ctx context.Context, token, osName, arch, channel, currentVersion string) (UpdateCheckResult, error) {
	q := url.Values{}
	q.Set("os", osName)
	q.Set("arch", arch)
	q.Set("channel", channel)
	if currentVersion != "" {
		q.Set("currentVersion", currentVersion)
	}
	var out UpdateCheckResult
	err := c.do(ctx, http.MethodGet, "/v1/updates/check?"+q.Encode(), token, nil, &out)
	return out, err
}

// Notification is one platform notification targeted at this installation.
type Notification struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// ListNotifications returns this installation's active, unacknowledged notifications.
func (c *Client) ListNotifications(ctx context.Context, token string) ([]Notification, error) {
	out := []Notification{}
	err := c.do(ctx, http.MethodGet, "/v1/notifications", token, nil, &out)
	return out, err
}

// Ack marks a notification as read for this installation.
func (c *Client) Ack(ctx context.Context, token, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/notifications/"+url.PathEscape(id)+"/ack", token, nil, nil)
}

// DownloadArtifact streams downloadURL (relative to BaseURL, or absolute) to destPath,
// verifying it against expectedSHA256 as it's written. The partial file is removed on
// any failure, including a checksum mismatch.
func (c *Client) DownloadArtifact(ctx context.Context, token, downloadURL, destPath, expectedSHA256 string, onProgress func(done, total int64)) (err error) {
	full := downloadURL
	if !strings.HasPrefix(full, "http://") && !strings.HasPrefix(full, "https://") {
		full = c.BaseURL + downloadURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := DownloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return &StatusError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(destPath)
		}
	}()

	hasher := sha256.New()
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			hasher.Write(buf[:n])
			done += int64(n)
			if onProgress != nil {
				onProgress(done, resp.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(sum, expectedSHA256) {
		return fmt.Errorf("downloaded file checksum %s does not match expected %s", sum, expectedSHA256)
	}
	return nil
}
