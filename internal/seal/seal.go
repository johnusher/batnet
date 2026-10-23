// Package seal encrypts traffic between a village's nodes with keys derived
// from the village key (XChaCha20-Poly1305, HKDF-SHA256).
package seal

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// PurposeMesh is the key purpose for traffic over the WiFi mesh.
const PurposeMesh = "batnet mesh v1"

// PurposePlaces is the key purpose for the radios' places hubs share (B70).
const PurposePlaces = "batnet places v1"

// ErrOpen means the box wasn't made with this village's key, or was damaged.
var ErrOpen = errors.New("seal: cannot open (wrong village key or damaged)")

// VillageKey is the shared secret every house in a village holds.
type VillageKey [32]byte

// NewVillageKey makes a random key for a new village.
func NewVillageKey() (VillageKey, error) {
	var k VillageKey
	_, err := rand.Read(k[:])
	return k, err
}

// Derive returns a 32-byte key for one purpose, so keys for the mesh, LoRa and
// the like never coincide.
func (k VillageKey) Derive(purpose string) [32]byte {
	var out [32]byte
	if _, err := io.ReadFull(hkdf.New(sha256.New, k[:], nil, []byte(purpose)), out[:]); err != nil {
		panic(err) // HKDF-SHA256 can always produce 32 bytes
	}
	return out
}

// Sealer seals and opens boxes for one purpose.
type Sealer struct{ aead cipher.AEAD }

// NewSealer returns a Sealer for k and purpose.
func NewSealer(k VillageKey, purpose string) (*Sealer, error) {
	key := k.Derive(purpose)
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal returns nonce || ciphertext.
func (s *Sealer) Seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, chacha20poly1305.NonceSizeX, chacha20poly1305.NonceSizeX+len(plain)+s.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, plain, nil), nil
}

// Open reverses Seal.
func (s *Sealer) Open(box []byte) ([]byte, error) {
	if len(box) < chacha20poly1305.NonceSizeX+s.aead.Overhead() {
		return nil, ErrOpen
	}
	plain, err := s.aead.Open(nil, box[:chacha20poly1305.NonceSizeX], box[chacha20poly1305.NonceSizeX:], nil)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
