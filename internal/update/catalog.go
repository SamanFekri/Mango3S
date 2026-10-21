package update

// Catalog of the web platform routes the desktop app talks to (the public
// ones, /v1/* and /downloads/*; web/docs/API_CONTRACT.md), with an example
// request for each, so the API console can send any of them one at a time.
// The admin dashboard's routes (/api/*) are left out: they are for the web
// dashboard, not the app. Example bodies follow the zod schemas in
// web/packages/contracts. Keep this list in step with the API contract.
//
// Paths, queries and bodies may contain {variables}; the console fills them
// from its variables (installationId, accountToken, clientId, …), which start
// from this installation's registration and are updated from responses.

// Auth says which credentials a request carries.
const (
	AuthNone         = "none"         // public
	AuthInstallation = "installation" // Authorization: Bearer {installationToken}
	AuthAccount      = "account"      // Authorization: Bearer {accountToken}
	AuthDevice       = "device"       // X-Installation-Token: {installationToken} (optional, sign-in)
)

// Endpoint is one route of the web API.
type Endpoint struct {
	ID          string `json:"id"`
	Group       string `json:"group"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Query       string `json:"query,omitempty"`
	Body        string `json:"body,omitempty"`
	Auth        string `json:"auth"`
	Writes      bool   `json:"writes"`
	Description string `json:"description"`
	// Captures maps response JSON fields to console variables ("token" →
	// "accountToken").
	Captures map[string]string `json:"captures,omitempty"`
}

const (
	gPublic  = "Public"
	gInstall = "App: installation"
	gAccount = "App: account & sync"
)

func ep(id, group, method, path, auth, name, desc string) Endpoint {
	return Endpoint{ID: id, Group: group, Name: name, Method: method, Path: path, Auth: auth, Description: desc, Writes: method != "GET"}
}

func (e Endpoint) q(query string) Endpoint { e.Query = query; return e }
func (e Endpoint) b(body string) Endpoint  { e.Body = body; return e }
func (e Endpoint) capture(kv ...string) Endpoint {
	e.Captures = map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		e.Captures[kv[i]] = kv[i+1]
	}
	return e
}

// APICatalog lists every route.
var APICatalog = []Endpoint{
	// ---- Public ----
	ep("health", gPublic, "GET", "/health", AuthNone, "Health check", "Is the API up?"),
	ep("app-info", gPublic, "GET", "/v1/app-info", AuthNone, "App info", "About / Support links and donation settings."),
	ep("downloads.releases", gPublic, "GET", "/downloads/releases", AuthNone, "Published releases", "Releases on the public download page."),
	ep("downloads.page", gPublic, "GET", "/downloads/page", AuthNone, "Download page data", "Everything the public download page shows."),
	ep("downloads.artifact", gPublic, "GET", "/downloads/{artifactId}", AuthNone, "Download an artifact", "Fetches an installer (only its size is shown)."),
	ep("public.privacy", gPublic, "GET", "/public/privacy", AuthNone, "Privacy policy (website)", "The published policy for the website."),
	ep("public.consent", gPublic, "POST", "/public/consent", AuthNone, "Record cookie consent", "The website's cookie banner answer.").
		b(`{"analytics": false, "marketing": false}`),

	// ---- App: installation (Bearer installation token) ----
	ep("installations.register", gInstall, "POST", "/v1/installations/register", AuthNone, "Register installation", "First check-in: returns the installation ID and token.").
		b(`{"deviceFingerprint": "{deviceFingerprint}", "appVersion": "{appVersion}", "os": "{os}", "arch": "{arch}", "label": "api-console"}`).
		capture("installationId", "installationId", "installationToken", "installationToken"),
	ep("installations.heartbeat", gInstall, "POST", "/v1/installations/{installationId}/heartbeat", AuthInstallation, "Heartbeat", "Activity ping (the app sends one every 10 minutes).").
		b(`{"appVersion": "{appVersion}", "os": "{os}", "arch": "{arch}"}`),
	ep("updates.check", gInstall, "GET", "/v1/updates/check", AuthInstallation, "Check for updates", "Newest release for this OS and architecture.").
		q("os={os}&arch={arch}&channel=stable&currentVersion={appVersion}").capture("latest.artifact.id", "artifactId"),
	ep("notifications.list", gInstall, "GET", "/v1/notifications", AuthInstallation, "List notifications", "Announcements waiting for this installation.").
		capture("0.id", "notificationId"),
	ep("notifications.ack", gInstall, "POST", "/v1/notifications/{notificationId}/ack", AuthInstallation, "Acknowledge notification", "Marks it read."),
	ep("notifications.respond", gInstall, "POST", "/v1/notifications/{notificationId}/respond", AuthInstallation, "Answer notification", `Reports the pressed button ("close" for the X).`).
		b(`{"buttonId": "close"}`),
	ep("privacy.get", gInstall, "GET", "/v1/policies/privacy", AuthInstallation, "Privacy policy", "Current policy and whether this installation accepted it.").
		capture("version", "policyVersion"),
	ep("privacy.accept", gInstall, "POST", "/v1/policies/privacy/accept", AuthInstallation, "Accept privacy policy", "Records acceptance of a version.").
		b(`{"version": {policyVersion}}`),

	// ---- App: account & sync (Bearer account token) ----
	ep("account.register", gAccount, "POST", "/v1/account/register", AuthNone, "Sign up: send code", "Emails a 6-digit code.").
		b(`{"email": "{email}", "password": "{password}"}`),
	ep("account.verify", gAccount, "POST", "/v1/account/register/verify", AuthDevice, "Sign up: verify code", "Creates the account and signs in.").
		b(`{"email": "{email}", "code": "{code}", "password": "{password}", "deviceLabel": "api-console"}`).capture("accountToken", "accountToken"),
	ep("account.login", gAccount, "POST", "/v1/account/login", AuthDevice, "Sign in", "Returns an account token (ca_…).").
		b(`{"email": "{email}", "password": "{password}", "deviceLabel": "api-console"}`).capture("accountToken", "accountToken"),
	ep("account.logout", gAccount, "POST", "/v1/account/logout", AuthAccount, "Sign out", "Ends this account session."),
	ep("account.get", gAccount, "GET", "/v1/account", AuthAccount, "Account", "Profile, machines and sync status."),
	ep("account.password.change", gAccount, "POST", "/v1/account/password/change", AuthAccount, "Change password", "").
		b(`{"currentPassword": "{password}", "newPassword": "{newPassword}"}`),
	ep("account.password.forgot", gAccount, "POST", "/v1/account/password/forgot", AuthNone, "Forgot password", "Emails a reset code.").
		b(`{"email": "{email}"}`),
	ep("account.password.reset", gAccount, "POST", "/v1/account/password/reset", AuthDevice, "Reset password", "With the emailed code and the recovery key (or discardSyncedData: true).").
		b(`{"email": "{email}", "code": "{code}", "newPassword": "{newPassword}", "recoveryKey": "{recoveryKey}"}`).capture("accountToken", "accountToken"),
	ep("account.recovery.rotate", gAccount, "POST", "/v1/account/recovery-key/rotate", AuthAccount, "New recovery key", "Replaces the recovery key.").
		b(`{"password": "{password}"}`).capture("recoveryKey", "recoveryKey"),
	ep("account.delete", gAccount, "POST", "/v1/account/delete", AuthAccount, "Delete account", "Deletes the account and its synced data.").
		b(`{"password": "{password}"}`),
	ep("sync.transport-key", gAccount, "GET", "/v1/account/sync/transport-key", AuthNone, "Sync: transport key", "The key the app seals sync items and passwords to."),
	ep("sync.push", gAccount, "PUT", "/v1/account/sync/items", AuthAccount, "Sync: push items", "Uploads items (the app sends them sealed to the transport key).").
		b(`{"items": [{"key": "api-console-test", "data": null, "deleted": true, "updatedAt": "2026-01-01T00:00:00.000Z"}]}`),
	ep("sync.pull", gAccount, "POST", "/v1/account/sync/pull", AuthAccount, "Sync: pull items", "Downloads and decrypts items with the password.").
		b(`{"password": "{password}"}`),
	ep("sync.wipe", gAccount, "POST", "/v1/account/sync/wipe", AuthAccount, "Sync: erase everything", "Erases all synced items on the server.").
		b(`{"password": "{password}"}`),
}

// FindEndpoint returns the catalog entry with id.
func FindEndpoint(id string) (Endpoint, bool) {
	for _, e := range APICatalog {
		if e.ID == id {
			return e, true
		}
	}
	return Endpoint{}, false
}
