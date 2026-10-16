package config

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
)

func TestBuildURI(t *testing.T) {
	cases := []struct {
		s    ServerConfig
		want string
	}{
		{ServerConfig{Type: TypeStandalone, Hosts: []HostPort{{"db.local", 27017}}}, "mongodb://db.local:27017/"},
		{ServerConfig{Type: TypeReplicaSet, ReplicaSet: "rs0", Hosts: []HostPort{{"a", 27017}, {"b", 27018}}}, "mongodb://a:27017,b:27018/?replicaSet=rs0"},
		{ServerConfig{Type: TypeSharded, Hosts: []HostPort{{"m1", 27017}, {"m2", 27017}}}, "mongodb://m1:27017,m2:27017/"},
		{ServerConfig{Type: TypeSRV, Hosts: []HostPort{{"cluster0.example.net", 0}}}, "mongodb+srv://cluster0.example.net/"},
		{ServerConfig{Type: TypeReplicaSet, DirectConnection: true, Hosts: []HostPort{{"::1", 27017}}}, "mongodb://[::1]:27017/?directConnection=true"},
	}
	for _, c := range cases {
		got, err := BuildURI(c.s)
		if err != nil || got != c.want {
			t.Errorf("BuildURI(%+v) = %q, %v; want %q", c.s, got, err, c.want)
		}
	}
	bad := []ServerConfig{
		{Type: TypeStandalone},
		{Type: TypeStandalone, Hosts: []HostPort{{"a", 1}, {"b", 2}}},
		{Type: TypeReplicaSet, Hosts: []HostPort{{"a", 27017}}},
		{Type: TypeSRV, Hosts: []HostPort{{"x.net", 27017}}},
		{Type: TypeStandalone, Hosts: []HostPort{{"a", 70000}}},
		{Type: TypeStandalone, Hosts: []HostPort{{"a/b", 1}}},
	}
	for _, s := range bad {
		if _, err := BuildURI(s); !apperr.Is(err, apperr.CodeInvalidInput) {
			t.Errorf("BuildURI(%+v) should be rejected, got %v", s, err)
		}
	}
}

func TestParseURIRoundTrip(t *testing.T) {
	p, err := ParseURI("mongodb://alice:secret@a:27017,b:27018/shop?replicaSet=rs0&authSource=admin&tls=true&appName=x&maxPoolSize=5")
	if err != nil {
		t.Fatal(err)
	}
	if p.Server.Type != TypeReplicaSet || p.Server.ReplicaSet != "rs0" || len(p.Server.Hosts) != 2 || p.Server.Hosts[1].Port != 27018 {
		t.Fatalf("server: %+v", p.Server)
	}
	if p.Username != "alice" || !p.HasPassword || p.AuthSource != "admin" || !p.TLS || p.Database != "shop" {
		t.Fatalf("parsed: %+v", p)
	}
	if strings.Join(p.Unmapped, ",") != "maxPoolSize" {
		t.Fatalf("unmapped options: %v", p.Unmapped)
	}
	uri, err := BuildURI(p.Server)
	if err != nil || uri != "mongodb://a:27017,b:27018/?replicaSet=rs0" {
		t.Fatalf("rebuild: %q %v", uri, err)
	}
	srv, err := ParseURI("mongodb+srv://u@cluster0.example.net/?retryWrites=true")
	if err != nil && !strings.Contains(err.Error(), "lookup") {
		t.Fatal(err)
	}
	if err == nil && (srv.Server.Type != TypeSRV || srv.Server.Hosts[0].Host != "cluster0.example.net") {
		t.Fatalf("srv: %+v", srv.Server)
	}
	if _, err := ParseURI("http://nope"); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func valid() Profile {
	return Profile{Name: "x", URI: "mongodb://localhost:27017"}.Normalize()
}

func TestProfileValidationNamesTheTab(t *testing.T) {
	cases := []struct {
		mut  func(*Profile)
		want string
	}{
		{func(p *Profile) { p.Name = " " }, "Server:"},
		{func(p *Profile) {
			p.SSH = SSHConfig{Enabled: true, Port: 22, AuthMethod: SSHAuthKey, Host: "h", Username: "u"}
		}, "SSH: choose a private key"},
		{func(p *Profile) { p.SSH = SSHConfig{Enabled: true, Port: 22, AuthMethod: SSHAuthPassword} }, "SSH:"},
		{func(p *Profile) { p.Proxy = ProxyConfig{Type: ProxySOCKS5, Port: 1080} }, "Proxy:"},
		{func(p *Profile) { p.Proxy = ProxyConfig{Type: "socks4", Host: "h", Port: 1} }, "Proxy: unsupported"},
		{func(p *Profile) { p.AuthMechanism = MechX509 }, "SSL / TLS tab"},
		{func(p *Profile) { p.AuthMechanism = MechSCRAM256 }, "needs a username"},
		{func(p *Profile) { p.Advanced.ReadPreference = "fastest" }, "Advanced: unknown read preference"},
		{func(p *Profile) { p.Advanced.ReadPreferenceTags = "dc:ny" }, "other than primary"},
		{func(p *Profile) { p.Advanced.ReadPreference = "secondary"; p.Advanced.MaxStalenessSeconds = 10 }, "at least 90"},
		{func(p *Profile) { p.Advanced.MinPoolSize = 10; p.Advanced.MaxPoolSize = 5 }, "min pool size"},
		{func(p *Profile) { p.Advanced.Compressors = []string{"lz4"} }, "unsupported compressor"},
		{func(p *Profile) { p.Advanced.WriteConcernW = "0"; p.Advanced.WriteConcernJournal = "true" }, "journaling"},
	}
	for _, c := range cases {
		p := valid()
		c.mut(&p)
		err := p.Normalize().Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want error containing %q, got %v", c.want, err)
		}
	}
	p := valid()
	p.AuthMechanism = MechGSSAPI
	p.Username = "u"
	if err := p.Validate(); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("Kerberos must be reported as unsupported, got %v", err)
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
}

func TestNormalizeDefaults(t *testing.T) {
	p := Profile{Name: " a ", Folder: " prod / eu /", SSH: SSHConfig{Enabled: true}, Proxy: ProxyConfig{Type: ProxyHTTP}}.Normalize()
	if p.Name != "a" || p.Folder != "prod/eu" || p.SSH.Port != 22 || p.SSH.AuthMethod != SSHAuthPassword || p.Proxy.Port != 8080 ||
		p.Server.Mode != ServerModeURI || p.ConnectTimeoutSeconds != 10 {
		t.Fatalf("defaults: %+v", p)
	}
}

func TestSecretFlags(t *testing.T) {
	var p Profile
	p.SetSecretFlag(SecretSSHPassword, true)
	p.SetSecretFlag(SecretProxyPassword, true)
	p.SetSecretFlag(SecretSSHPassword, false)
	p.SetSecretFlag(SecretPassword, true)
	if !p.HasPassword || p.HasSecret(SecretSSHPassword) || !p.HasSecret(SecretProxyPassword) {
		t.Fatalf("flags: %+v", p)
	}
	if SecretKey("p1", SecretPassword) != "p1" || SecretKey("p1", SecretSSHPassword) != "p1:sshPassword" {
		t.Fatal("secret keys")
	}
}

func TestStoreKeepsNewSectionsAndLastConnected(t *testing.T) {
	s := NewStore(t.TempDir())
	p, err := s.SaveProfile(Profile{Name: "a", URI: "mongodb://h", Folder: "prod", SSH: SSHConfig{Enabled: true, Host: "bastion", Username: "me", AuthMethod: SSHAuthAgent},
		Advanced: AdvancedConfig{ReadPreference: "nearest", Compressors: []string{"zstd"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchConnected(p.ID, p.CreatedAt); err != nil {
		t.Fatal(err)
	}
	p.Name = "renamed"
	p.LastConnectedAt = nil
	if _, err := s.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProfile(p.ID)
	if got.Name != "renamed" || got.Folder != "prod" || got.SSH.Host != "bastion" || got.SSH.Port != 22 ||
		got.Advanced.Compressors[0] != "zstd" || got.LastConnectedAt == nil {
		t.Fatalf("round trip: %+v", got)
	}
}
