package update

// Catalog of every route the web platform API serves (web/apps/api/src/app.ts
// and its modules), with an example request for each, so the API console can
// send any of them one at a time. Example bodies follow the zod schemas in
// web/packages/contracts. Keep this list in step with the web API.
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
	AuthAdmin        = "admin"        // dashboard session cookie from POST /api/auth/login
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
	Multipart   bool   `json:"multipart,omitempty"` // file upload: not sendable from the console
	Description string `json:"description"`
	// Captures maps response JSON fields to console variables ("token" →
	// "accountToken"); "$cookie" captures the session cookie.
	Captures map[string]string `json:"captures,omitempty"`
}

const (
	gPublic   = "Public"
	gInstall  = "App: installation"
	gAccount  = "App: account & sync"
	gAuth     = "Admin: sign-in"
	gRoles    = "Admin: roles & users"
	gClients  = "Admin: clients & installations"
	gReleases = "Admin: releases & downloads"
	gNotif    = "Admin: notifications"
	gPolicies = "Admin: privacy policy"
	gSettings = "Admin: settings & email"
	gBackups  = "Admin: backups"
	gReports  = "Admin: analytics & audit"
)

func ep(id, group, method, path, auth, name, desc string) Endpoint {
	return Endpoint{ID: id, Group: group, Name: name, Method: method, Path: path, Auth: auth, Description: desc, Writes: method != "GET"}
}

func (e Endpoint) q(query string) Endpoint { e.Query = query; return e }
func (e Endpoint) b(body string) Endpoint  { e.Body = body; return e }
func (e Endpoint) multipart() Endpoint     { e.Multipart = true; return e }
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
	ep("sync.push", gAccount, "PUT", "/v1/account/sync/items", AuthAccount, "Sync: push items", "Uploads encrypted items (data is the app's ciphertext).").
		b(`{"items": [{"key": "api-console-test", "data": null, "deleted": true, "updatedAt": "2026-01-01T00:00:00.000Z"}]}`),
	ep("sync.pull", gAccount, "POST", "/v1/account/sync/pull", AuthAccount, "Sync: pull items", "Downloads and decrypts items with the password.").
		b(`{"password": "{password}"}`),
	ep("sync.wipe", gAccount, "POST", "/v1/account/sync/wipe", AuthAccount, "Sync: erase everything", "Erases all synced items on the server.").
		b(`{"password": "{password}"}`),

	// ---- Admin: sign-in (session cookie) ----
	ep("admin.bootstrap-status", gAuth, "GET", "/api/auth/bootstrap-status", AuthNone, "Setup needed?", "Whether the first Owner account still has to be created."),
	ep("admin.bootstrap", gAuth, "POST", "/api/auth/bootstrap", AuthNone, "Create first Owner", "Only works once, on a fresh server.").
		b(`{"username": "{adminUser}", "email": "{adminEmail}", "password": "{adminPassword}"}`).capture("$cookie", "adminCookie"),
	ep("admin.login", gAuth, "POST", "/api/auth/login", AuthNone, "Admin sign in", "Starts a dashboard session; the console keeps its cookie.").
		b(`{"identifier": "{adminUser}", "password": "{adminPassword}"}`).capture("$cookie", "adminCookie"),
	ep("admin.logout", gAuth, "POST", "/api/auth/logout", AuthAdmin, "Admin sign out", ""),
	ep("admin.me", gAuth, "GET", "/api/auth/me", AuthAdmin, "Current admin", "Who the session belongs to, with permissions."),
	ep("admin.sessions", gAuth, "GET", "/api/auth/sessions", AuthAdmin, "Admin sessions", "").capture("0.id", "sessionId"),
	ep("admin.sessions.delete", gAuth, "DELETE", "/api/auth/sessions/{sessionId}", AuthAdmin, "End a session", ""),
	ep("admin.invites", gAuth, "GET", "/api/auth/invites", AuthAdmin, "Invites", "").capture("0.id", "inviteId"),
	ep("admin.invites.create", gAuth, "POST", "/api/auth/invites", AuthAdmin, "Create invite", "").
		b(`{"roleId": "{roleId}", "note": "api-console", "expiresInHours": 24}`).capture("id", "inviteId", "token", "inviteToken"),
	ep("admin.invites.delete", gAuth, "DELETE", "/api/auth/invites/{inviteId}", AuthAdmin, "Revoke invite", ""),
	ep("admin.invites.accept", gAuth, "POST", "/api/auth/invites/accept", AuthNone, "Accept invite", "Creates the invited admin.").
		b(`{"token": "{inviteToken}", "username": "new-admin", "email": "new-admin@example.com", "password": "{adminPassword}"}`),

	// ---- Admin: roles & users ----
	ep("admin.permissions", gRoles, "GET", "/api/permissions", AuthAdmin, "Permissions", "Every permission a role can have."),
	ep("admin.roles", gRoles, "GET", "/api/roles", AuthAdmin, "Roles", "").capture("0.id", "roleId"),
	ep("admin.roles.create", gRoles, "POST", "/api/roles", AuthAdmin, "Create role", "").
		b(`{"name": "API console", "description": "Created from the Mango3S API console", "permissions": []}`).capture("id", "roleId"),
	ep("admin.roles.update", gRoles, "PATCH", "/api/roles/{roleId}", AuthAdmin, "Update role", "").b(`{"description": "Updated"}`),
	ep("admin.roles.delete", gRoles, "DELETE", "/api/roles/{roleId}", AuthAdmin, "Delete role", ""),
	ep("admin.users", gRoles, "GET", "/api/admin-users", AuthAdmin, "Admin users", "").capture("0.id", "adminUserId"),
	ep("admin.users.update", gRoles, "PATCH", "/api/admin-users/{adminUserId}", AuthAdmin, "Update admin user", "").b(`{"status": "active"}`),
	ep("admin.users.delete", gRoles, "DELETE", "/api/admin-users/{adminUserId}", AuthAdmin, "Delete admin user", ""),

	// ---- Admin: clients & installations ----
	ep("admin.clients", gClients, "GET", "/api/clients", AuthAdmin, "Clients", "").q("page=1&pageSize=25").capture("items.0.id", "clientId"),
	ep("admin.clients.create", gClients, "POST", "/api/clients", AuthAdmin, "Create client", "").
		b(`{"displayName": "API console", "note": "Created from the Mango3S API console"}`).capture("id", "clientId"),
	ep("admin.clients.get", gClients, "GET", "/api/clients/{clientId}", AuthAdmin, "Client", ""),
	ep("admin.clients.update", gClients, "PATCH", "/api/clients/{clientId}", AuthAdmin, "Update client", "").b(`{"note": "Updated"}`),
	ep("admin.clients.rotate-key", gClients, "POST", "/api/clients/{clientId}/rotate-key", AuthAdmin, "Rotate client key", ""),
	ep("admin.clients.installations", gClients, "GET", "/api/clients/{clientId}/installations", AuthAdmin, "Client's installations", ""),
	ep("admin.clients.identity", gClients, "GET", "/api/clients/{clientId}/identity", AuthAdmin, "Client identity", "Account and machines."),
	ep("admin.clients.machines.add", gClients, "POST", "/api/clients/{clientId}/machines", AuthAdmin, "Add machine", "").
		b(`{"machineId": "{deviceFingerprint}", "label": "api-console"}`).capture("id", "machineRowId"),
	ep("admin.clients.machines.delete", gClients, "DELETE", "/api/clients/{clientId}/machines/{machineRowId}", AuthAdmin, "Remove machine", ""),
	ep("admin.clients.account.sign-out", gClients, "POST", "/api/clients/{clientId}/account/sign-out", AuthAdmin, "Sign out client account", "Ends every session of the client's account."),
	ep("admin.clients.account.sync.delete", gClients, "DELETE", "/api/clients/{clientId}/account/sync", AuthAdmin, "Erase client's synced data", ""),
	ep("admin.installations", gClients, "GET", "/api/installations", AuthAdmin, "Installations", "").q("page=1&pageSize=25"),
	ep("admin.installations.update", gClients, "PATCH", "/api/installations/{installationId}", AuthAdmin, "Update installation", "Block or unblock (status), or rename.").
		b(`{"status": "active"}`),

	// ---- Admin: releases & downloads ----
	ep("admin.releases", gReleases, "GET", "/api/releases", AuthAdmin, "Releases", "").capture("items.0.id", "releaseId"),
	ep("admin.releases.create", gReleases, "POST", "/api/releases", AuthAdmin, "Create release", "").
		b(`{"version": "{appVersion}", "channel": "stable", "notes": "Created from the API console"}`).capture("id", "releaseId"),
	ep("admin.releases.get", gReleases, "GET", "/api/releases/{releaseId}", AuthAdmin, "Release", "").capture("artifacts.0.id", "artifactId"),
	ep("admin.releases.update", gReleases, "PATCH", "/api/releases/{releaseId}", AuthAdmin, "Update release", "").b(`{"notes": "Updated"}`),
	ep("admin.releases.publish", gReleases, "POST", "/api/releases/{releaseId}/publish", AuthAdmin, "Publish release", ""),
	ep("admin.releases.withdraw", gReleases, "POST", "/api/releases/{releaseId}/withdraw", AuthAdmin, "Withdraw release", ""),
	ep("admin.releases.rollback", gReleases, "POST", "/api/releases/{releaseId}/rollback", AuthAdmin, "Roll back release", ""),
	ep("admin.releases.delete", gReleases, "DELETE", "/api/releases/{releaseId}", AuthAdmin, "Delete release", ""),
	ep("admin.releases.artifacts.upload", gReleases, "POST", "/api/releases/{releaseId}/artifacts", AuthAdmin, "Upload artifact", "File upload (multipart): use the dashboard, or Add artifact from URL.").multipart(),
	ep("admin.releases.artifacts.url", gReleases, "POST", "/api/releases/{releaseId}/artifacts/url", AuthAdmin, "Add artifact from URL", "The server downloads the file.").
		b(`{"os": "{os}", "arch": "{arch}", "format": "zip", "url": "https://example.com/Mango3S.zip"}`).capture("id", "artifactId"),
	ep("admin.releases.artifacts.delete", gReleases, "DELETE", "/api/releases/{releaseId}/artifacts/{artifactId}", AuthAdmin, "Delete artifact", ""),
	ep("admin.download-links", gReleases, "GET", "/api/download-links", AuthAdmin, "Download links", "").capture("0.id", "linkId"),
	ep("admin.download-links.create", gReleases, "POST", "/api/download-links", AuthAdmin, "Create download link", "").
		b(`{"source": "external", "os": "{os}", "label": "API console", "url": "https://example.com/download"}`).capture("id", "linkId"),
	ep("admin.download-links.reorder", gReleases, "POST", "/api/download-links/reorder", AuthAdmin, "Reorder download links", "").b(`{"ids": ["{linkId}"]}`),
	ep("admin.download-links.replace", gReleases, "PUT", "/api/download-links/{linkId}", AuthAdmin, "Replace download link", "").
		b(`{"source": "external", "os": "{os}", "label": "API console (edited)", "url": "https://example.com/download"}`),
	ep("admin.download-links.enable", gReleases, "PATCH", "/api/download-links/{linkId}", AuthAdmin, "Enable / disable link", "").b(`{"enabled": false}`),
	ep("admin.download-links.delete", gReleases, "DELETE", "/api/download-links/{linkId}", AuthAdmin, "Delete download link", ""),

	// ---- Admin: notifications ----
	ep("admin.notifications", gNotif, "GET", "/api/notifications", AuthAdmin, "Notifications", "").capture("items.0.id", "notificationId"),
	ep("admin.notifications.create", gNotif, "POST", "/api/notifications", AuthAdmin, "Create notification", "Draft by default; set status with Update.").
		b(`{"title": "API console test", "body": "Sent from the Mango3S API console.", "audience": {"type": "all"}, "buttons": [{"label": "OK", "action": "ok"}]}`).capture("id", "notificationId"),
	ep("admin.notifications.update", gNotif, "PATCH", "/api/notifications/{notificationId}", AuthAdmin, "Update notification", "").b(`{"status": "published"}`),
	ep("admin.notifications.delete", gNotif, "DELETE", "/api/notifications/{notificationId}", AuthAdmin, "Delete notification", ""),

	// ---- Admin: privacy policy ----
	ep("admin.policies", gPolicies, "GET", "/api/policies/privacy", AuthAdmin, "Policy versions", "").capture("0.id", "policyId"),
	ep("admin.policies.create", gPolicies, "POST", "/api/policies/privacy", AuthAdmin, "Create draft", "").
		b(`{"title": "Privacy policy", "body": "# Privacy\n\nDraft from the API console.", "summary": "Test draft"}`).capture("id", "policyId"),
	ep("admin.policies.update", gPolicies, "PATCH", "/api/policies/privacy/{policyId}", AuthAdmin, "Update draft", "").b(`{"summary": "Updated"}`),
	ep("admin.policies.delete", gPolicies, "DELETE", "/api/policies/privacy/{policyId}", AuthAdmin, "Delete draft", ""),
	ep("admin.policies.publish", gPolicies, "POST", "/api/policies/privacy/{policyId}/publish", AuthAdmin, "Publish policy", "").
		b(`{"requireAcceptance": true, "announce": false}`),
	ep("admin.policies.consent-stats", gPolicies, "GET", "/api/policies/consent-stats", AuthAdmin, "Consent statistics", ""),

	// ---- Admin: settings & email ----
	ep("admin.settings", gSettings, "GET", "/api/settings", AuthAdmin, "Platform settings", ""),
	ep("admin.settings.update", gSettings, "PATCH", "/api/settings", AuthAdmin, "Update settings", "Any subset of the settings.").b(`{"mascotEnabled": true}`),
	ep("admin.settings.test-email", gSettings, "POST", "/api/settings/test-email", AuthAdmin, "Send test email", "").b(`{"to": "{adminEmail}"}`),
	ep("admin.email-templates", gSettings, "GET", "/api/settings/email-templates", AuthAdmin, "Email templates", "").capture("0.id", "templateId"),
	ep("admin.email-templates.update", gSettings, "PUT", "/api/settings/email-templates/{templateId}", AuthAdmin, "Save email template", "").
		b(`{"subject": "Subject", "html": "<p>Hello</p>", "text": "Hello"}`),
	ep("admin.email-templates.reset", gSettings, "DELETE", "/api/settings/email-templates/{templateId}", AuthAdmin, "Reset email template", "Back to the default."),
	ep("admin.email-templates.test", gSettings, "POST", "/api/settings/email-templates/{templateId}/test", AuthAdmin, "Send template test", "").b(`{"to": "{adminEmail}"}`),

	// ---- Admin: backups ----
	ep("admin.backups", gBackups, "GET", "/api/backups", AuthAdmin, "Backups", "").capture("0.id", "backupId"),
	ep("admin.backups.create", gBackups, "POST", "/api/backups", AuthAdmin, "Create backup", "").capture("id", "backupId"),
	ep("admin.backups.get", gBackups, "GET", "/api/backups/{backupId}", AuthAdmin, "Backup", ""),
	ep("admin.backups.download", gBackups, "GET", "/api/backups/{backupId}/download", AuthAdmin, "Download backup", "Only its size is shown."),
	ep("admin.backups.restore", gBackups, "POST", "/api/backups/{backupId}/restore", AuthAdmin, "Restore backup", "Overwrites the live database.").b(`{"confirm": "RESTORE"}`),
	ep("admin.backups.restore-upload", gBackups, "POST", "/api/backups/restore-upload", AuthAdmin, "Restore uploaded backup", "File upload (multipart): use the dashboard.").multipart(),

	// ---- Admin: analytics & audit ----
	ep("admin.analytics", gReports, "GET", "/api/analytics/overview", AuthAdmin, "Analytics overview", "").q("days=30"),
	ep("admin.audit", gReports, "GET", "/api/audit-logs", AuthAdmin, "Audit log", "").q("page=1&pageSize=25"),
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
