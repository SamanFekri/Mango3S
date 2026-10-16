package mongox

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/v2/tag"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// Credentials are the secrets a profile may need. They come from the
// credential store and are only ever held in memory.
type Credentials struct {
	Password        string
	SSHPassword     string
	SSHPassphrase   string
	ProxyPassword   string
	TLSKeyPassword  string
	AWSSessionToken string
}

// BuildClientOptions turns a profile and its secrets into driver options.
// Explicit form settings override the same options in the connection string.
// It does not dial; SSH and proxy dialers are added by open.
func BuildClientOptions(p config.Profile, c Credentials) (*options.ClientOptions, error) {
	p = p.Normalize()
	o := options.Client().ApplyURI(p.URI)
	if err := o.Validate(); err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Server: invalid connection string.")
	}
	adv := p.Advanced
	switch {
	case adv.AppName != "":
		o.SetAppName(adv.AppName)
	case o.AppName == nil:
		o.SetAppName("Mango3S")
	}
	timeout := time.Duration(p.ConnectTimeoutSeconds) * time.Second
	o.SetConnectTimeout(timeout)
	if adv.ServerSelectionTimeoutSeconds > 0 {
		o.SetServerSelectionTimeout(time.Duration(adv.ServerSelectionTimeoutSeconds) * time.Second)
	} else {
		o.SetServerSelectionTimeout(timeout)
	}
	if err := applyAuth(o, p, c); err != nil {
		return nil, err
	}
	if err := applyTLS(o, p.TLS, c.TLSKeyPassword); err != nil {
		return nil, err
	}
	if err := applyAdvanced(o, adv); err != nil {
		return nil, err
	}
	return o, nil
}

func applyAuth(o *options.ClientOptions, p config.Profile, c Credentials) error {
	if p.Username == "" && p.AuthMechanism == "" {
		return nil
	}
	cred := options.Credential{}
	if o.Auth != nil {
		cred = *o.Auth
	}
	if p.Username != "" {
		cred.Username = p.Username
		cred.Password = c.Password
		cred.PasswordSet = c.Password != ""
	}
	if p.AuthSource != "" {
		cred.AuthSource = p.AuthSource
	} else if cred.AuthSource == "" {
		// The driver ignores authSource when the URI itself has no user.
		if cs, err := connstring.Parse(p.URI); err == nil {
			cred.AuthSource = cs.AuthSource
		}
	}
	if p.AuthMechanism != "" {
		cred.AuthMechanism = p.AuthMechanism
	}
	props, err := parseProps(p.AuthProperties)
	if err != nil {
		return err
	}
	if p.AuthMechanism == config.MechAWS && c.AWSSessionToken != "" {
		props["AWS_SESSION_TOKEN"] = c.AWSSessionToken
	}
	if len(props) > 0 {
		if cred.AuthMechanismProperties == nil {
			cred.AuthMechanismProperties = map[string]string{}
		}
		for k, v := range props {
			cred.AuthMechanismProperties[k] = v
		}
	}
	o.SetAuth(cred)
	return nil
}

func parseProps(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		k, v, ok := strings.Cut(kv, ":")
		if !ok {
			return nil, apperr.Invalid("Authentication: mechanism properties must look like KEY:value,KEY2:value2.")
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// applyTLS validates the certificate files with actionable messages, then
// lets the driver build its own tls.Config from them so behavior (encrypted
// keys, hostname checks) matches the tls* connection string options exactly.
func applyTLS(o *options.ClientOptions, t config.TLSConfig, keyPassword string) error {
	if !t.Enabled && t.CAFile == "" && t.CertKeyFile == "" {
		return nil
	}
	if !t.Enabled {
		return apperr.Invalid("SSL / TLS: certificate files are set but TLS is off. Enable TLS or clear the files.")
	}
	if err := CheckTLSFiles(t, keyPassword); err != nil {
		return err
	}
	q := url.Values{}
	q.Set("tls", "true")
	if t.CAFile != "" {
		q.Set("tlsCAFile", t.CAFile)
	}
	if t.CertKeyFile != "" {
		q.Set("tlsCertificateKeyFile", t.CertKeyFile)
		if keyPassword != "" {
			q.Set("tlsCertificateKeyFilePassword", keyPassword)
		}
	}
	tmp := options.Client().ApplyURI("mongodb://localhost/?" + q.Encode())
	if err := tmp.Validate(); err != nil {
		// Never echo the URI: it can contain the key password.
		msg := err.Error()
		if keyPassword != "" {
			msg = strings.ReplaceAll(msg, keyPassword, "•••")
		}
		return apperr.New(apperr.CodeTLS, "SSL / TLS: the certificate files could not be loaded: %s", msg)
	}
	cfg := tmp.TLSConfig
	switch {
	case t.AllowInvalidCertificates:
		cfg.InsecureSkipVerify = true
	case t.AllowInvalidHostnames:
		// The Go driver has no tlsAllowInvalidHostnames: verify the chain
		// ourselves and skip only the host name check.
		roots := cfg.RootCAs
		cfg.InsecureSkipVerify = true
		cfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("tls: the server sent no certificate")
			}
			inter := x509.NewCertPool()
			for _, c := range cs.PeerCertificates[1:] {
				inter.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter})
			return err
		}
	}
	o.SetTLSConfig(cfg)
	return nil
}

// CheckTLSFiles reports missing, unreadable, empty, expired or encrypted
// certificate files before the driver gets a chance to fail obscurely.
func CheckTLSFiles(t config.TLSConfig, keyPassword string) error {
	if t.CAFile != "" {
		data, err := readPEM("CA file", t.CAFile)
		if err != nil {
			return err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: %s contains no PEM certificates.", t.CAFile)
		}
	}
	if t.CertKeyFile != "" {
		data, err := readPEM("client certificate", t.CertKeyFile)
		if err != nil {
			return err
		}
		var cert *x509.Certificate
		hasKey, encrypted := false, false
		for rest := data; ; {
			var b *pem.Block
			b, rest = pem.Decode(rest)
			if b == nil {
				break
			}
			switch {
			case b.Type == "CERTIFICATE" && cert == nil:
				if c, err := x509.ParseCertificate(b.Bytes); err == nil {
					cert = c
				}
			case strings.Contains(b.Type, "PRIVATE KEY"):
				hasKey = true
				//nolint:staticcheck // legacy encrypted PEM is still common for client keys
				encrypted = b.Type == "ENCRYPTED PRIVATE KEY" || x509.IsEncryptedPEMBlock(b)
			}
		}
		if cert == nil {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: %s has no certificate. It must contain the client certificate and its private key.", t.CertKeyFile)
		}
		if !hasKey {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: %s has no private key. Concatenate the certificate and key into one PEM file.", t.CertKeyFile)
		}
		if encrypted && keyPassword == "" {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: the private key in %s is encrypted. Enter its password.", t.CertKeyFile)
		}
		now := time.Now()
		if now.After(cert.NotAfter) {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: the client certificate expired on %s.", cert.NotAfter.Format("2006-01-02"))
		}
		if now.Before(cert.NotBefore) {
			return apperr.New(apperr.CodeTLS, "SSL / TLS: the client certificate is not valid until %s.", cert.NotBefore.Format("2006-01-02"))
		}
	}
	return nil
}

func readPEM(what, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, apperr.New(apperr.CodeTLS, "SSL / TLS: the %s %s does not exist.", what, path)
	case err != nil:
		return nil, apperr.Wrap(apperr.CodeTLS, err, "SSL / TLS: the "+what+" can't be read.")
	}
	return data, nil
}

func applyAdvanced(o *options.ClientOptions, a config.AdvancedConfig) error {
	if a.OperationTimeoutSeconds > 0 {
		o.SetTimeout(time.Duration(a.OperationTimeoutSeconds) * time.Second)
	}
	if a.HeartbeatSeconds > 0 {
		o.SetHeartbeatInterval(time.Duration(a.HeartbeatSeconds) * time.Second)
	}
	if a.LocalThresholdMS > 0 {
		o.SetLocalThreshold(time.Duration(a.LocalThresholdMS) * time.Millisecond)
	}
	if a.ReadPreference != "" {
		mode, err := readpref.ModeFromString(a.ReadPreference)
		if err != nil {
			return apperr.Invalid("Advanced: %v", err)
		}
		var ro []readpref.Option
		tags, err := config.ParseTags(a.ReadPreferenceTags)
		if err != nil {
			return err
		}
		if len(tags) > 0 {
			ro = append(ro, readpref.WithTagSets(tag.NewTagSetFromMap(tags)))
		}
		if a.MaxStalenessSeconds > 0 {
			ro = append(ro, readpref.WithMaxStaleness(time.Duration(a.MaxStalenessSeconds)*time.Second))
		}
		rp, err := readpref.New(mode, ro...)
		if err != nil {
			return apperr.Invalid("Advanced: %v", err)
		}
		o.SetReadPreference(rp)
	}
	if a.ReadConcern != "" {
		o.SetReadConcern(&readconcern.ReadConcern{Level: a.ReadConcern})
	}
	if a.WriteConcernW != "" || a.WriteConcernJournal != "" {
		wc := &writeconcern.WriteConcern{}
		if o.WriteConcern != nil {
			*wc = *o.WriteConcern
		}
		if a.WriteConcernW != "" {
			if n, err := strconv.Atoi(a.WriteConcernW); err == nil {
				wc.W = n
			} else {
				wc.W = a.WriteConcernW
			}
		}
		if a.WriteConcernJournal != "" {
			j := a.WriteConcernJournal == "true"
			wc.Journal = &j
		}
		o.SetWriteConcern(wc)
	}
	if a.MaxPoolSize > 0 {
		o.SetMaxPoolSize(uint64(a.MaxPoolSize))
	}
	if a.MinPoolSize > 0 {
		o.SetMinPoolSize(uint64(a.MinPoolSize))
	}
	if a.MaxConnecting > 0 {
		o.SetMaxConnecting(uint64(a.MaxConnecting))
	}
	if a.MaxIdleTimeSeconds > 0 {
		o.SetMaxConnIdleTime(time.Duration(a.MaxIdleTimeSeconds) * time.Second)
	}
	if a.RetryWrites != "" {
		o.SetRetryWrites(a.RetryWrites == "true")
	}
	if a.RetryReads != "" {
		o.SetRetryReads(a.RetryReads == "true")
	}
	if len(a.Compressors) > 0 {
		o.SetCompressors(a.Compressors)
	}
	return nil
}
