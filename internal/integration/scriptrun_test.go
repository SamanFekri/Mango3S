//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/scriptrun"
)

// Runs a script through the real mongosh against the password-protected
// server (MONGO_AUTH_URI), and checks the password never shows up.
func TestMongoshScriptWithAuth(t *testing.T) {
	authURI := os.Getenv("MONGO_AUTH_URI")
	if authURI == "" {
		t.Skip("MONGO_AUTH_URI not set")
	}
	mongosh, err := exec.LookPath("mongosh")
	if err != nil {
		t.Skip("mongosh is not installed")
	}
	stripped, user, password, err := config.SplitCredentials(authURI)
	if err != nil || password == "" {
		t.Fatalf("MONGO_AUTH_URI needs a user and password: %v", err)
	}
	p := config.Profile{URI: stripped, Username: user}
	uri, secrets, err := scriptrun.ConnectionURI(p, mongox.Credentials{Password: password}, "mango3s_script_it")
	if err != nil {
		t.Fatal(err)
	}
	script := `
const n = 3;
let total = 0;
for (let i = 1; i <= n; i++) {
  db.items.insertOne({ i });
  total += i;
}
print("total", total, "count", db.items.countDocuments());
print("db", db.getName());
printjson(db.runCommand({ connectionStatus: 1 }).authInfo.authenticatedUsers.length);
db.dropDatabase();
throw new Error("boom " + "` + password + `");
`
	var lines []string
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	code, err := scriptrun.Run(ctx, mongosh, uri, script, secrets, func(l scriptrun.Line) { lines = append(lines, l.Stream+": "+l.Text) })
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(lines, "\n")
	if !strings.Contains(out, "total 6 count 3") || !strings.Contains(out, "db mango3s_script_it") || !strings.Contains(out, "1") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if code == 0 || !strings.Contains(out, "boom *****") {
		t.Fatalf("the thrown error should fail the run and be shown scrubbed (exit %d):\n%s", code, out)
	}
	if strings.Contains(out, password) {
		t.Fatalf("the password leaked into the output:\n%s", out)
	}
	if strings.Contains(out, "mango3s-mongosh-") {
		t.Fatalf("temporary paths leaked into the output:\n%s", out)
	}

	// Cancel stops a long script.
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() { time.Sleep(time.Second); cancel2() }()
	start := time.Now()
	if _, err := scriptrun.Run(ctx2, mongosh, uri, "while (true) { sleep(100) }", secrets, func(scriptrun.Line) {}); err == nil {
		t.Fatal("a canceled run must report it")
	}
	if time.Since(start) > 15*time.Second {
		t.Fatal("cancel did not stop mongosh")
	}
}
