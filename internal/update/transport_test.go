package update

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestTransportEnvelopeOpensOnlyForItsPurpose(t *testing.T) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	env, err := SealTo(priv.PublicKey(), []byte("secret"), AADItem("conn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := OpenWith(priv, env, AADItem("conn-1")); err != nil || string(got) != "secret" {
		t.Fatalf("open: %q, %v", got, err)
	}
	if _, err := OpenWith(priv, env, AADItem("conn-2")); err == nil {
		t.Fatal("opened for another item")
	}
	if _, err := OpenWith(priv, env, AADWipe); err == nil {
		t.Fatal("opened for another purpose")
	}
	ct, _ := base64.StdEncoding.DecodeString(env.Ct)
	ct[len(ct)-1] ^= 1
	env.Ct = base64.StdEncoding.EncodeToString(ct)
	if _, err := OpenWith(priv, env, AADItem("conn-1")); err == nil {
		t.Fatal("opened tampered data")
	}
}
