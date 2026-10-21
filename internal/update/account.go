package update

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Client accounts and encrypted sync (web/docs/API_CONTRACT.md §7). These are
// actions the user is waiting on, so each is a single attempt (no background
// retries) and errors go straight back to the UI.

// AccountMachine is one computer linked to the account.
type AccountMachine struct {
	MachineID  string  `json:"machineId"`
	Label      *string `json:"label"`
	LastSeenAt *string `json:"lastSeenAt"`
}

// AccountSync describes the account's synced data and the admin's limits.
type AccountSync struct {
	Enabled      bool    `json:"enabled"`
	ItemCount    int     `json:"itemCount"`
	LastSyncedAt *string `json:"lastSyncedAt"`
	MaxItems     int     `json:"maxItems"`
	MaxItemKB    int     `json:"maxItemKb"`
}

// Account is GET /v1/account.
type Account struct {
	ClientID             string           `json:"clientId"`
	Email                string           `json:"email"`
	CreatedAt            string           `json:"createdAt"`
	RecoveryKeyCreatedAt *string          `json:"recoveryKeyCreatedAt"`
	Machines             []AccountMachine `json:"machines"`
	Sync                 AccountSync      `json:"sync"`
}

// AccountSession is returned by sign-up, sign-in and password reset.
// RecoveryKey is only set on sign-up and on a reset that erased the data;
// DataKept only on a reset.
type AccountSession struct {
	AccountToken string  `json:"accountToken"`
	ExpiresAt    string  `json:"expiresAt"`
	Account      Account `json:"account"`
	RecoveryKey  string  `json:"recoveryKey,omitempty"`
	DataKept     *bool   `json:"dataKept,omitempty"`
}

// CodeSent is the answer to "email me a code" requests.
type CodeSent struct {
	CodeSentTo       string `json:"codeSentTo"`
	ExpiresInMinutes int    `json:"expiresInMinutes"`
}

// SyncItem is one synced thing (one connection). Data is nil for a deletion.
// On upload SyncPush moves Data into Sealed, so only the envelope is sent.
type SyncItem struct {
	Key       string    `json:"key"`
	Data      *string   `json:"data"`
	Sealed    *Envelope `json:"sealed,omitempty"`
	Deleted   bool      `json:"deleted"`
	UpdatedAt string    `json:"updatedAt"`
}

// SyncPushResult lists which items the server stored and which it already
// had a newer (or equal) version of.
type SyncPushResult struct {
	Accepted []string `json:"accepted"`
	Skipped  []string `json:"skipped"`
}

// DeviceInfo links this computer to the account on verify / sign-in.
type DeviceInfo struct {
	InstallationToken string // sent as X-Installation-Token; optional
	Label             string
}

func (c *Client) accountCall(ctx context.Context, method, path, token string, body, out any) error {
	return c.send(ctx, call{method: method, path: "/v1/account" + path, token: token, body: body, out: out})
}

func deviceHeaders(d DeviceInfo) map[string]string {
	if d.InstallationToken == "" {
		return nil
	}
	return map[string]string{"X-Installation-Token": d.InstallationToken}
}

// AccountRegister starts sign-up: the server emails a 6-digit code.
func (c *Client) AccountRegister(ctx context.Context, email, password string) (CodeSent, error) {
	var out CodeSent
	err := c.accountCall(ctx, http.MethodPost, "/register", "", map[string]string{"email": email, "password": password}, &out)
	return out, err
}

// AccountVerify finishes sign-up with the emailed code (and the same password again).
func (c *Client) AccountVerify(ctx context.Context, email, code, password string, d DeviceInfo) (AccountSession, error) {
	var out AccountSession
	body := map[string]string{"email": email, "code": code, "password": password, "deviceLabel": d.Label}
	err := c.send(ctx, call{method: http.MethodPost, path: "/v1/account/register/verify", body: body, out: &out, headers: deviceHeaders(d)})
	return out, err
}

// AccountLogin signs in.
func (c *Client) AccountLogin(ctx context.Context, email, password string, d DeviceInfo) (AccountSession, error) {
	var out AccountSession
	body := map[string]string{"email": email, "password": password, "deviceLabel": d.Label}
	err := c.send(ctx, call{method: http.MethodPost, path: "/v1/account/login", body: body, out: &out, headers: deviceHeaders(d)})
	return out, err
}

// AccountLogout ends this session on the server.
func (c *Client) AccountLogout(ctx context.Context, token string) error {
	return c.accountCall(ctx, http.MethodPost, "/logout", token, nil, nil)
}

// GetAccount returns the signed-in account.
func (c *Client) GetAccount(ctx context.Context, token string) (Account, error) {
	var out Account
	err := c.accountCall(ctx, http.MethodGet, "", token, nil, &out)
	return out, err
}

// AccountChangePassword keeps synced data and signs out every other device.
func (c *Client) AccountChangePassword(ctx context.Context, token, current, next string) error {
	return c.accountCall(ctx, http.MethodPost, "/password/change", token, map[string]string{"currentPassword": current, "newPassword": next}, nil)
}

// AccountForgotPassword emails a reset code (the answer is the same whether or not the email has an account).
func (c *Client) AccountForgotPassword(ctx context.Context, email string) (CodeSent, error) {
	var out CodeSent
	err := c.accountCall(ctx, http.MethodPost, "/password/forgot", "", map[string]string{"email": email}, &out)
	return out, err
}

// ResetPasswordRequest resets a forgotten password. With RecoveryKey the
// synced data is kept; with DiscardSyncedData it is erased and a new recovery
// key is issued.
type ResetPasswordRequest struct {
	Email             string `json:"email"`
	Code              string `json:"code"`
	NewPassword       string `json:"newPassword"`
	RecoveryKey       string `json:"recoveryKey,omitempty"`
	DiscardSyncedData bool   `json:"discardSyncedData,omitempty"`
}

// AccountResetPassword finishes a password reset.
func (c *Client) AccountResetPassword(ctx context.Context, req ResetPasswordRequest) (AccountSession, error) {
	var out AccountSession
	err := c.accountCall(ctx, http.MethodPost, "/password/reset", "", req, &out)
	return out, err
}

// AccountRotateRecoveryKey issues a new recovery key; the old one stops working.
func (c *Client) AccountRotateRecoveryKey(ctx context.Context, token, password string) (string, error) {
	var out struct {
		RecoveryKey string `json:"recoveryKey"`
	}
	err := c.accountCall(ctx, http.MethodPost, "/recovery-key/rotate", token, map[string]string{"password": password}, &out)
	return out.RecoveryKey, err
}

// AccountDelete deletes the account and everything synced to it.
func (c *Client) AccountDelete(ctx context.Context, token, password string) error {
	return c.accountCall(ctx, http.MethodPost, "/delete", token, map[string]string{"password": password}, nil)
}

// SyncPush uploads changed and deleted items (no password needed: the server
// encrypts them to the account's public key). Newest updatedAt wins. Each
// item's Data is sealed to the transport key and never sent readable.
func (c *Client) SyncPush(ctx context.Context, token string, items []SyncItem) (SyncPushResult, error) {
	var out SyncPushResult
	err := c.withSealer(ctx, func(s *sealer) error {
		sealed := make([]SyncItem, len(items))
		for i, it := range items {
			sealed[i] = it
			if it.Data == nil || it.Deleted {
				continue
			}
			env, err := s.seal([]byte(*it.Data), AADItem(it.Key))
			if err != nil {
				return err
			}
			sealed[i].Data, sealed[i].Sealed = nil, &env
		}
		return c.accountCall(ctx, http.MethodPut, "/sync/items", token, map[string]any{"items": sealed}, &out)
	})
	return out, err
}

// SyncPull downloads and decrypts every item; it needs the account password.
// The password goes sealed, and the server seals the answer to a one-time key.
func (c *Client) SyncPull(ctx context.Context, token, password string) ([]SyncItem, error) {
	var items []SyncItem
	err := c.withSealer(ctx, func(s *sealer) error {
		respond, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		body, err := s.sealedPassword(password, respond, AADPull)
		if err != nil {
			return err
		}
		var out struct {
			Sealed *Envelope `json:"sealed"`
		}
		if err := c.accountCall(ctx, http.MethodPost, "/sync/pull", token, body, &out); err != nil {
			return err
		}
		if out.Sealed == nil {
			return errors.New("the server answered a sealed pull without sealing it")
		}
		plain, err := OpenWith(respond, *out.Sealed, AADPullResponse)
		if err != nil {
			return fmt.Errorf("the server's answer could not be opened: %w", err)
		}
		var answer struct {
			Items []SyncItem `json:"items"`
		}
		if err := json.Unmarshal(plain, &answer); err != nil {
			return err
		}
		items = answer.Items
		return nil
	})
	return items, err
}

// SyncWipe erases everything synced to the account on the server.
func (c *Client) SyncWipe(ctx context.Context, token, password string) error {
	return c.withSealer(ctx, func(s *sealer) error {
		body, err := s.sealedPassword(password, nil, AADWipe)
		if err != nil {
			return err
		}
		return c.accountCall(ctx, http.MethodPost, "/sync/wipe", token, body, nil)
	})
}
