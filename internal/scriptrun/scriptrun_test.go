package scriptrun

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
)

func TestConnectionURI(t *testing.T) {
	p := config.Profile{URI: "mongodb://db1:27017,db2:27017/?replicaSet=rs0", Username: "ada", AuthSource: "admin",
		TLS: config.TLSConfig{Enabled: true, CAFile: "/ca.pem", CertKeyFile: "/me.pem"}}
	uri, secrets, err := ConnectionURI(p, mongox.Credentials{Password: "p@ss/word", TLSKeyPassword: "keypw"}, "shop")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mongodb://ada:p%40ss%2Fword@db1:27017,db2:27017/shop?", "replicaSet=rs0", "authSource=admin", "tls=true", "tlsCAFile=%2Fca.pem", "appName=Mango3S"} {
		if !strings.Contains(uri, want) {
			t.Errorf("%s lacks %q", uri, want)
		}
	}
	joined := strings.Join(secrets, " ")
	if !strings.Contains(joined, "p@ss/word") || !strings.Contains(joined, "keypw") {
		t.Errorf("secrets to scrub: %v", secrets)
	}
}

func TestCanRun(t *testing.T) {
	if err := CanRun(config.Profile{SSH: config.SSHConfig{Enabled: true}}); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("ssh: %v", err)
	}
	if err := CanRun(config.Profile{Proxy: config.ProxyConfig{Type: config.ProxySOCKS5}}); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("proxy: %v", err)
	}
	if err := CanRun(config.Profile{}); err != nil {
		t.Fatal(err)
	}
}
