package api

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/connio"
	"mango3s/internal/secrets"
)

// ConnectionExport is the outcome of ExportConnections.
type ConnectionExport struct {
	Path            string `json:"path"` // empty when the user canceled
	Count           int    `json:"count"`
	SecretsIncluded bool   `json:"secretsIncluded"`
}

// ExportConnections asks where to save the chosen connections (all when ids
// is empty). Secrets are left out unless includeSecrets is set, and are then
// encrypted with the passphrase.
func (a *App) ExportConnections(ids []string, includeSecrets bool, passphrase string) (ConnectionExport, error) {
	out, err := a.exportConnections(ids, includeSecrets, passphrase)
	return out, a.encode("ExportConnections", err)
}

func (a *App) exportConnections(ids []string, includeSecrets bool, passphrase string) (ConnectionExport, error) {
	data, list, err := a.connectionsFile(ids, includeSecrets, passphrase)
	if err != nil {
		return ConnectionExport{}, err
	}
	name := "mango3s-connections.json"
	if len(list) == 1 {
		name = safeFileName(list[0].Name) + ".mango3s-connection.json"
	}
	path, err := a.saveFileDialog(wruntime.SaveDialogOptions{Title: "Export connections", DefaultFilename: name})
	if err != nil || path == "" {
		return ConnectionExport{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return ConnectionExport{}, apperr.Wrap(apperr.CodeInternal, err, "Could not write the file.")
	}
	a.log.Info("connections exported", "count", len(list), "secrets", includeSecrets)
	return ConnectionExport{Path: path, Count: len(list), SecretsIncluded: includeSecrets}, nil
}

// connectionsFile builds the export file content.
func (a *App) connectionsFile(ids []string, includeSecrets bool, passphrase string) ([]byte, []config.Profile, error) {
	all, err := a.store.ListProfiles()
	if err != nil {
		return nil, nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var list []config.Profile
	for _, p := range all {
		if len(want) == 0 || want[p.ID] {
			list = append(list, p)
		}
	}
	if len(list) == 0 {
		return nil, nil, apperr.Invalid("Choose at least one connection to export.")
	}
	var sec connio.Secrets
	if includeSecrets {
		if len([]rune(passphrase)) < connio.MinPassphrase {
			return nil, nil, apperr.Invalid("Use a passphrase of at least %d characters to protect the exported passwords.", connio.MinPassphrase)
		}
		sec = connio.Secrets{}
		for _, p := range list {
			for _, kind := range config.SecretKinds {
				if !p.HasSecret(kind) {
					continue
				}
				v, err := a.secrets.Get(config.SecretKey(p.ID, kind))
				if errors.Is(err, secrets.ErrNotFound) {
					continue
				}
				if err != nil {
					return nil, nil, apperr.Wrap(apperr.CodeSecretBackend, err, fmt.Sprintf("A saved secret of %q could not be read from the secret store.", p.Name))
				}
				if sec[p.ID] == nil {
					sec[p.ID] = map[string]string{}
				}
				sec[p.ID][kind] = v
			}
		}
	}
	data, err := connio.Export(list, sec, passphrase, time.Now())
	if err != nil {
		return nil, nil, apperr.Invalid("%s", upperFirst(err.Error())+".")
	}
	return data, list, nil
}

func safeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}

// ImportItem is one connection found in a file, as shown before importing.
type ImportItem struct {
	Index       int      `json:"index"`
	Name        string   `json:"name"`
	Folder      string   `json:"folder"`
	Hosts       string   `json:"hosts"`
	Via         string   `json:"via"` // SSH / proxy summary
	SecretKinds []string `json:"secretKinds"`
	Conflict    string   `json:"conflict"`   // "" | "id" (same connection exists) | "name"
	ExistingID  string   `json:"existingId"` // the profile it conflicts with
	Notes       []string `json:"notes"`
	Error       string   `json:"error"`
}

// ImportPreview is the content of a connections file.
type ImportPreview struct {
	Path            string       `json:"path"`
	Format          string       `json:"format"`
	HasSecrets      bool         `json:"hasSecrets"`
	SecretsUnlocked bool         `json:"secretsUnlocked"`
	Items           []ImportItem `json:"items"`
	Warnings        []string     `json:"warnings"`
}

// ImportChoice says what to do with one item.
type ImportChoice struct {
	Index  int    `json:"index"`
	Action string `json:"action"` // add | replace | skip
	Name   string `json:"name"`   // add: the name to use
	Folder string `json:"folder"`
}

// ImportOutcome summarizes an import.
type ImportOutcome struct {
	Added    int      `json:"added"`
	Replaced int      `json:"replaced"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// ChooseConnectionsFile opens a file picker for a connections file.
func (a *App) ChooseConnectionsFile() (string, error) {
	p, err := a.openFileDialog("file", wruntime.OpenDialogOptions{Title: "Import connections", ShowHiddenFiles: true})
	return p, a.encode("ChooseConnectionsFile", err)
}

func readConnectionsFile(path string, passphrase string) (connio.Parsed, error) {
	f, err := os.Open(path)
	if err != nil {
		return connio.Parsed{}, apperr.Wrap(apperr.CodeInvalidInput, err, "The file cannot be opened.")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, connio.MaxFileSize+1))
	if err != nil {
		return connio.Parsed{}, apperr.Wrap(apperr.CodeInvalidInput, err, "The file cannot be read.")
	}
	p, err := connio.Parse(data, passphrase)
	if errors.Is(err, connio.ErrPassphrase) {
		return p, apperr.Invalid("The passphrase is wrong (or the file was modified).")
	}
	if err != nil {
		return p, apperr.Wrap(apperr.CodeInvalidInput, err, upperFirst(err.Error())+".")
	}
	return p, nil
}

// PreviewConnectionImport reads a connections file and reports conflicts
// with saved connections. Nothing is saved.
func (a *App) PreviewConnectionImport(path, passphrase string) (ImportPreview, error) {
	parsed, err := readConnectionsFile(path, passphrase)
	if err != nil {
		return ImportPreview{}, a.encode("PreviewConnectionImport", err)
	}
	existing, err := a.store.ListProfiles()
	if err != nil {
		return ImportPreview{}, a.encode("PreviewConnectionImport", err)
	}
	out := ImportPreview{Path: path, Format: parsed.Format, HasSecrets: parsed.HasSecrets, SecretsUnlocked: parsed.SecretsUnlocked, Warnings: parsed.Warnings, Items: []ImportItem{}}
	for i, c := range parsed.Candidates {
		it := ImportItem{Index: i, Name: c.Profile.Name, Folder: c.Profile.Folder, Hosts: hostsSummary(c.Profile.URI), SecretKinds: c.Kinds, Notes: c.Notes, Error: c.Error}
		if it.SecretKinds == nil {
			it.SecretKinds = []string{}
		}
		if c.Profile.SSH.Enabled {
			it.Via = "SSH " + c.Profile.SSH.Host
		} else if c.Profile.Proxy.Type != config.ProxyNone {
			it.Via = c.Profile.Proxy.Type + " proxy " + c.Profile.Proxy.Host
		}
		for _, e := range existing {
			if c.Profile.ID != "" && e.ID == c.Profile.ID {
				it.Conflict, it.ExistingID = "id", e.ID
				break
			}
			if strings.EqualFold(e.Name, c.Profile.Name) && e.Folder == c.Profile.Folder && it.Conflict == "" {
				it.Conflict, it.ExistingID = "name", e.ID
			}
		}
		out.Items = append(out.Items, it)
	}
	return out, nil
}

// hostsSummary lists hosts without credentials.
func hostsSummary(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Host
}

// ApplyConnectionImport saves the chosen connections. Passwords and other
// secrets go to the OS credential store, never into the settings file.
func (a *App) ApplyConnectionImport(path, passphrase string, choices []ImportChoice) (ImportOutcome, error) {
	out, err := a.applyConnectionImport(path, passphrase, choices)
	return out, a.encode("ApplyConnectionImport", err)
}

func (a *App) applyConnectionImport(path, passphrase string, choices []ImportChoice) (ImportOutcome, error) {
	parsed, err := readConnectionsFile(path, passphrase)
	if err != nil {
		return ImportOutcome{}, err
	}
	out := ImportOutcome{Errors: []string{}}
	for _, ch := range choices {
		if ch.Index < 0 || ch.Index >= len(parsed.Candidates) {
			return out, apperr.Invalid("The file changed since it was previewed; preview it again.")
		}
		c := parsed.Candidates[ch.Index]
		if ch.Action == "skip" {
			out.Skipped++
			continue
		}
		if c.Error != "" {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %s", c.Profile.Name, c.Error))
			continue
		}
		in := inputFromProfile(c.Profile)
		switch ch.Action {
		case "replace":
			if _, err := a.store.GetProfile(c.Profile.ID); err != nil || c.Profile.ID == "" {
				out.Errors = append(out.Errors, fmt.Sprintf("%s: the connection to replace no longer exists.", c.Profile.Name))
				continue
			}
		case "add":
			in.ID = ""
			if strings.TrimSpace(ch.Name) != "" {
				in.Name = strings.TrimSpace(ch.Name)
			}
		default:
			return out, apperr.Invalid("Unknown import action %q.", ch.Action)
		}
		if ch.Folder != "" || ch.Action == "add" {
			in.Folder = ch.Folder
		}
		in.Password = c.Secrets[config.SecretPassword]
		in.Secrets = map[string]SecretInput{}
		for k, v := range c.Secrets {
			if k != config.SecretPassword {
				in.Secrets[k] = SecretInput{Value: v}
			}
		}
		if _, err := a.saveProfile(in); err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %s", in.Name, apperr.From(err).Message))
			continue
		}
		if ch.Action == "replace" {
			out.Replaced++
		} else {
			out.Added++
		}
	}
	a.log.Info("connections imported", "format", parsed.Format, "added", out.Added, "replaced", out.Replaced, "skipped", out.Skipped, "errors", len(out.Errors))
	return out, nil
}

func inputFromProfile(p config.Profile) ProfileInput {
	return ProfileInput{
		ID: p.ID, Name: p.Name, Folder: p.Folder, URI: p.URI, Server: p.Server, Username: p.Username,
		AuthSource: p.AuthSource, AuthMechanism: p.AuthMechanism, AuthProperties: p.AuthProperties,
		ConnectTimeoutSeconds: p.ConnectTimeoutSeconds, ReadOnly: p.ReadOnly, TLS: p.TLS, SSH: p.SSH, Proxy: p.Proxy,
		Shell: p.Shell, Tools: p.Tools, Advanced: p.Advanced,
	}
}

// ConnectionURI returns a connection's connection string for copying. The
// password is included only when explicitly asked for.
func (a *App) ConnectionURI(id string, withPassword bool) (string, error) {
	p, err := a.store.GetProfile(id)
	if err != nil {
		return "", a.encode("ConnectionURI", err)
	}
	u, err := url.Parse(p.URI)
	if err != nil {
		return p.URI, nil
	}
	if p.Username != "" {
		u.User = url.User(p.Username)
		if withPassword && p.HasPassword {
			pw, err := a.secrets.Get(config.SecretKey(p.ID, config.SecretPassword))
			if err != nil && !errors.Is(err, secrets.ErrNotFound) {
				return "", a.encode("ConnectionURI", apperr.Wrap(apperr.CodeSecretBackend, err, "The saved password could not be read."))
			}
			if pw != "" {
				u.User = url.UserPassword(p.Username, pw)
			}
		}
	}
	q := u.Query()
	if p.AuthSource != "" && q.Get("authSource") == "" {
		q.Set("authSource", p.AuthSource)
	}
	if p.AuthMechanism != "" && q.Get("authMechanism") == "" {
		q.Set("authMechanism", p.AuthMechanism)
	}
	if p.TLS.Enabled && q.Get("tls") == "" && q.Get("ssl") == "" {
		q.Set("tls", "true")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
