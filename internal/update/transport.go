package update

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Transport encryption for sync (web/docs/API_CONTRACT.md §7, "Transport
// encryption"). Every item, and the password sent to pull or wipe, is sealed
// to the server's X25519 transport key before it leaves this computer; the
// server opens it and stores items encrypted to the account key. A pull's
// answer comes back sealed to a one-time key sent with the request. So
// connection data never crosses the wire readable, even where HTTPS is opened
// on the way.
//
//	key = HKDF-SHA256(X25519(one-time, recipient), salt = epk | recipient,
//	                  info = "mango3s-transport-v1"); AES-256-GCM with the
//	purpose ("mango3s-transport:item:<key>", ":pull", ":wipe",
//	":pull-response") as additional data.

const transportInfo = "mango3s-transport-v1"

// Additional data (purpose) for each kind of envelope.
const (
	AADPull         = "mango3s-transport:pull"
	AADPullResponse = "mango3s-transport:pull-response"
	AADWipe         = "mango3s-transport:wipe"
)

func AADItem(key string) string { return "mango3s-transport:item:" + key }

// Envelope is something sealed to a recipient's X25519 key. Kid names the
// server transport key it was sealed to (empty on the server's answers).
type Envelope struct {
	Kid string `json:"kid,omitempty"`
	Epk string `json:"epk"`
	Ct  string `json:"ct"`
}

// TransportKey is GET /v1/account/sync/transport-key.
type TransportKey struct {
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
}

const transportAlgorithm = "x25519-hkdf-sha256-aes256gcm"

// ErrTransportUnsupported means the server can't take sealed sync requests
// (it's older than transport encryption). Nothing is sent in the clear instead.
var ErrTransportUnsupported = errors.New("the Mango3S server does not support encrypted sync yet")

// GetTransportKey fetches the key sync requests are sealed to.
func (c *Client) GetTransportKey(ctx context.Context) (TransportKey, error) {
	var out TransportKey
	err := c.accountCall(ctx, http.MethodGet, "/sync/transport-key", "", nil, &out)
	var se *StatusError
	if errors.As(err, &se) && se.Status == http.StatusNotFound {
		return out, ErrTransportUnsupported
	}
	if err != nil {
		return out, err
	}
	if out.Algorithm != transportAlgorithm {
		return out, fmt.Errorf("%w (unknown algorithm %q)", ErrTransportUnsupported, out.Algorithm)
	}
	return out, nil
}

func (k TransportKey) public() (*ecdh.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(k.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("transport key: %w", err)
	}
	return ecdh.X25519().NewPublicKey(raw)
}

func transportAEAD(shared, epk, recipient []byte) (cipher.AEAD, error) {
	salt := append(append([]byte{}, epk...), recipient...)
	key, err := hkdf.Key(sha256.New, shared, salt, transportInfo, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// SealTo seals plaintext to recipient. Ciphertext layout: iv | tag | data,
// as the server's AES-GCM helpers use.
func SealTo(recipient *ecdh.PublicKey, plaintext []byte, aad string) (Envelope, error) {
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Envelope{}, err
	}
	shared, err := eph.ECDH(recipient)
	if err != nil {
		return Envelope{}, err
	}
	epk := eph.PublicKey().Bytes()
	aead, err := transportAEAD(shared, epk, recipient.Bytes())
	if err != nil {
		return Envelope{}, err
	}
	iv := make([]byte, aead.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return Envelope{}, err
	}
	sealed := aead.Seal(nil, iv, plaintext, []byte(aad)) // data | tag
	body, tag := sealed[:len(sealed)-aead.Overhead()], sealed[len(sealed)-aead.Overhead():]
	ct := append(append(append([]byte{}, iv...), tag...), body...)
	return Envelope{Epk: base64.StdEncoding.EncodeToString(epk), Ct: base64.StdEncoding.EncodeToString(ct)}, nil
}

// OpenWith opens an envelope sealed to priv.
func OpenWith(priv *ecdh.PrivateKey, env Envelope, aad string) ([]byte, error) {
	epk, err := base64.StdEncoding.DecodeString(env.Epk)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(env.Ct)
	if err != nil {
		return nil, err
	}
	peer, err := ecdh.X25519().NewPublicKey(epk)
	if err != nil {
		return nil, err
	}
	shared, err := priv.ECDH(peer)
	if err != nil {
		return nil, err
	}
	aead, err := transportAEAD(shared, epk, priv.PublicKey().Bytes())
	if err != nil {
		return nil, err
	}
	n, t := aead.NonceSize(), aead.Overhead()
	if len(ct) < n+t {
		return nil, errors.New("sealed answer is too short")
	}
	iv, tag, body := ct[:n], ct[n:n+t], ct[n+t:]
	return aead.Open(nil, iv, append(append([]byte{}, body...), tag...), []byte(aad))
}

// sealer seals to one fetched transport key.
type sealer struct {
	key TransportKey
	pub *ecdh.PublicKey
}

func (c *Client) newSealer(ctx context.Context) (*sealer, error) {
	k, err := c.GetTransportKey(ctx)
	if err != nil {
		return nil, err
	}
	pub, err := k.public()
	if err != nil {
		return nil, err
	}
	return &sealer{key: k, pub: pub}, nil
}

func (s *sealer) seal(plaintext []byte, aad string) (Envelope, error) {
	env, err := SealTo(s.pub, plaintext, aad)
	env.Kid = s.key.KeyID
	return env, err
}

func isKeyChanged(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == "transport_key_changed"
}

// withSealer runs fn with the current transport key, and once more with a
// fresh one if the server replaced its key in between.
func (c *Client) withSealer(ctx context.Context, fn func(*sealer) error) error {
	for attempt := 0; ; attempt++ {
		s, err := c.newSealer(ctx)
		if err != nil {
			return err
		}
		if err = fn(s); attempt == 0 && isKeyChanged(err) {
			continue
		}
		return err
	}
}

// sealedPassword builds a pull or wipe body. For a pull, respond is the
// one-time key the answer is sealed to.
func (s *sealer) sealedPassword(password string, respond *ecdh.PrivateKey, aad string) (map[string]any, error) {
	payload := map[string]string{"password": password}
	if respond != nil {
		payload["responseKey"] = base64.StdEncoding.EncodeToString(respond.PublicKey().Bytes())
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env, err := s.seal(plain, aad)
	if err != nil {
		return nil, err
	}
	return map[string]any{"sealed": env}, nil
}
