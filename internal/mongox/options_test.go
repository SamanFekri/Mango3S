package mongox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

func TestAdvancedOptionsOverrideURI(t *testing.T) {
	p := config.Profile{Name: "x", URI: "mongodb://h/?appName=fromURI&maxPoolSize=7", ConnectTimeoutSeconds: 4,
		Advanced: config.AdvancedConfig{
			AppName: "reporting", ServerSelectionTimeoutSeconds: 9, OperationTimeoutSeconds: 30, HeartbeatSeconds: 15,
			ReadPreference: "secondaryPreferred", ReadPreferenceTags: "dc:ny", MaxStalenessSeconds: 120,
			ReadConcern: "majority", WriteConcernW: "majority", WriteConcernJournal: "true",
			MaxPoolSize: 20, MinPoolSize: 2, MaxIdleTimeSeconds: 60, RetryWrites: "false", Compressors: []string{"zstd", "snappy"},
		}}
	o, err := BuildClientOptions(p, Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if *o.AppName != "reporting" || *o.ConnectTimeout != 4*time.Second || *o.ServerSelectionTimeout != 9*time.Second ||
		*o.Timeout != 30*time.Second || *o.HeartbeatInterval != 15*time.Second {
		t.Fatalf("timeouts/app name: %v %v %v %v", *o.AppName, *o.ConnectTimeout, *o.ServerSelectionTimeout, *o.Timeout)
	}
	if o.ReadPreference.Mode() != readpref.SecondaryPreferredMode || len(o.ReadPreference.TagSets()) != 1 {
		t.Fatalf("read preference: %v", o.ReadPreference)
	}
	if ms, ok := o.ReadPreference.MaxStaleness(); !ok || ms != 120*time.Second {
		t.Fatal("max staleness")
	}
	if o.ReadConcern.Level != "majority" || o.WriteConcern.W != "majority" || !*o.WriteConcern.Journal {
		t.Fatalf("concerns: %+v %+v", o.ReadConcern, o.WriteConcern)
	}
	if *o.MaxPoolSize != 20 || *o.MinPoolSize != 2 || *o.MaxConnIdleTime != time.Minute || *o.RetryWrites || strings.Join(o.Compressors, ",") != "zstd,snappy" {
		t.Fatal("pool/retry/compressors")
	}
	// Numeric w is sent as a number.
	p.Advanced = config.AdvancedConfig{WriteConcernW: "2"}
	o, _ = BuildClientOptions(p, Credentials{})
	if o.WriteConcern.W != 2 || *o.AppName != "fromURI" || *o.MaxPoolSize != 7 {
		t.Fatalf("w=2 and URI options kept: %+v %v", o.WriteConcern, *o.AppName)
	}
}

func TestAuthMechanismsAndProperties(t *testing.T) {
	o, err := BuildClientOptions(config.Profile{URI: "mongodb://h", Username: "AKIA", AuthMechanism: config.MechAWS},
		Credentials{Password: "secret", AWSSessionToken: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Auth.AuthMechanism != "MONGODB-AWS" || o.Auth.Password != "secret" || o.Auth.AuthMechanismProperties["AWS_SESSION_TOKEN"] != "tok" {
		t.Fatalf("aws: %+v", o.Auth)
	}
	o, err = BuildClientOptions(config.Profile{URI: "mongodb://h", AuthMechanism: config.MechOIDC, AuthProperties: "ENVIRONMENT:azure,TOKEN_RESOURCE:api://x"}, Credentials{})
	if err != nil || o.Auth.AuthMechanismProperties["ENVIRONMENT"] != "azure" || o.Auth.AuthMechanismProperties["TOKEN_RESOURCE"] != "api://x" {
		t.Fatalf("oidc: %+v %v", o.Auth, err)
	}
	if _, err := BuildClientOptions(config.Profile{URI: "mongodb://h", AuthMechanism: config.MechOIDC, AuthProperties: "oops"}, Credentials{}); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("bad properties: %v", err)
	}
}

// writeCert creates a self-signed certificate (and key) valid in [from, to].
func writeCert(t *testing.T, dir, name string, from, to time.Time, withKey bool) string {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "client"}, NotBefore: from, NotAfter: to,
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if withKey {
		kb, _ := x509.MarshalECPrivateKey(key)
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})...)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTLSFilesAreValidatedWithClearMessages(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	good := writeCert(t, dir, "client.pem", now.Add(-time.Hour), now.Add(time.Hour), true)
	ca := writeCert(t, dir, "ca.pem", now.Add(-time.Hour), now.Add(time.Hour), false)
	expired := writeCert(t, dir, "old.pem", now.Add(-48*time.Hour), now.Add(-24*time.Hour), true)
	noKey := writeCert(t, dir, "nokey.pem", now.Add(-time.Hour), now.Add(time.Hour), false)
	empty := filepath.Join(dir, "empty.pem")
	_ = os.WriteFile(empty, []byte("not a cert"), 0o600)

	base := config.Profile{URI: "mongodb://h"}
	cases := []struct {
		tls  config.TLSConfig
		want string
	}{
		{config.TLSConfig{Enabled: true, CAFile: filepath.Join(dir, "missing.pem")}, "does not exist"},
		{config.TLSConfig{Enabled: true, CAFile: empty}, "contains no PEM certificates"},
		{config.TLSConfig{Enabled: true, CertKeyFile: expired}, "expired on"},
		{config.TLSConfig{Enabled: true, CertKeyFile: noKey}, "has no private key"},
		{config.TLSConfig{CAFile: ca}, "TLS is off"},
	}
	for _, c := range cases {
		p := base
		p.TLS = c.tls
		_, err := BuildClientOptions(p, Credentials{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: want %q, got %v", c.tls, c.want, err)
		}
	}
	p := base
	p.TLS = config.TLSConfig{Enabled: true, CAFile: ca, CertKeyFile: good, AllowInvalidHostnames: true}
	o, err := BuildClientOptions(p, Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if o.TLSConfig == nil || o.TLSConfig.RootCAs == nil || len(o.TLSConfig.Certificates) != 1 {
		t.Fatalf("tls config not built: %+v", o.TLSConfig)
	}
	p.TLS = config.TLSConfig{Enabled: true, AllowInvalidCertificates: true}
	o, _ = BuildClientOptions(p, Credentials{})
	if !o.TLSConfig.InsecureSkipVerify {
		t.Fatal("allow invalid certificates must skip verification")
	}
}

func TestTLSErrorsAreClassified(t *testing.T) {
	for msg, want := range map[string]string{
		"server selection error: x509: certificate signed by unknown authority":           "CA file",
		"x509: certificate is valid for a.example, not localhost":                         "host name",
		"connection(h:27017) ... first record does not look like a TLS handshake":         "does not speak TLS",
		"connection() error occurred during connection handshake: remote error: tls: bad": "client certificate",
	} {
		e := apperr.From(errString(msg))
		if e.Code != apperr.CodeTLS || !strings.Contains(e.Message, want) {
			t.Errorf("%q → %s %q", msg, e.Code, e.Message)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestAllowInvalidHostnamesStillVerifiesTheChain(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	ca := writeCert(t, dir, "ca.pem", now.Add(-time.Hour), now.Add(time.Hour), false)
	o, err := BuildClientOptions(config.Profile{URI: "mongodb://h", TLS: config.TLSConfig{Enabled: true, CAFile: ca, AllowInvalidHostnames: true}}, Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if !o.TLSConfig.InsecureSkipVerify || o.TLSConfig.VerifyConnection == nil {
		t.Fatal("hostname-only relaxation must install a chain verifier")
	}
	// A certificate not signed by the CA is still rejected.
	other := writeCert(t, dir, "other.pem", now.Add(-time.Hour), now.Add(time.Hour), false)
	data, _ := os.ReadFile(other)
	b, _ := pem.Decode(data)
	cert, _ := x509.ParseCertificate(b.Bytes)
	if err := o.TLSConfig.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}); err == nil {
		t.Fatal("an untrusted certificate must fail verification")
	}
	data, _ = os.ReadFile(ca)
	b, _ = pem.Decode(data)
	caCert, _ := x509.ParseCertificate(b.Bytes)
	if err := o.TLSConfig.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{caCert}}); err != nil {
		t.Fatalf("a certificate from the CA must pass regardless of host name: %v", err)
	}
}
