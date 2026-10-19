package update

import (
	"context"
	"errors"
	"net/http"
)

// Donation is the optional "support the project" link the admin configured.
type Donation struct {
	URL         string `json:"url"`
	Heading     string `json:"heading"`
	Message     string `json:"message"`
	ButtonLabel string `json:"buttonLabel"`
}

// AppInfo is GET /v1/app-info: links for the About / Support screen.
// Donation is nil when the admin turned donations off.
type AppInfo struct {
	Name             string    `json:"name"`
	WebsiteURL       string    `json:"websiteUrl"`
	DownloadURL      string    `json:"downloadUrl"`
	PrivacyPolicyURL string    `json:"privacyPolicyUrl"`
	ContactURL       string    `json:"contactUrl"`
	Donation         *Donation `json:"donation"`
}

// GetAppInfo fetches the About / Support links. No token is needed.
func (c *Client) GetAppInfo(ctx context.Context) (AppInfo, error) {
	var out AppInfo
	err := c.do(ctx, http.MethodGet, "/v1/app-info", "", nil, &out)
	return out, err
}

// PrivacyPolicy is GET /v1/policies/privacy for this installation.
type PrivacyPolicy struct {
	Version            int     `json:"version"`
	Title              string  `json:"title"`
	Body               string  `json:"body"` // Markdown
	Summary            *string `json:"summary"`
	PublishedAt        *string `json:"publishedAt"`
	URL                string  `json:"url"`
	AcceptanceRequired bool    `json:"acceptanceRequired"`
	AcceptedAt         *string `json:"acceptedAt"`
}

// GetPrivacyPolicy returns the current policy and whether this installation must accept it.
func (c *Client) GetPrivacyPolicy(ctx context.Context, token string) (PrivacyPolicy, error) {
	var out PrivacyPolicy
	err := c.do(ctx, http.MethodGet, "/v1/policies/privacy", token, nil, &out)
	return out, err
}

// AcceptPrivacyPolicy records acceptance of version. A 409 (IsConflict) means
// a newer version was published meanwhile: fetch it and show that one.
func (c *Client) AcceptPrivacyPolicy(ctx context.Context, token string, version int) error {
	return c.send(ctx, call{method: http.MethodPost, path: "/v1/policies/privacy/accept", token: token, body: map[string]int{"version": version}})
}

// IsConflict reports whether err is a 409.
func IsConflict(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == http.StatusConflict
}
