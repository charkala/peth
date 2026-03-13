// Package wallet provides encrypted wallet key management for peth.
package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Sentinel errors for wallet operations.
var (
	ErrWalletNotFound = errors.New("wallet not found")
	ErrWalletExists   = errors.New("wallet already exists")
)

// Key represents a wallet key with its metadata.
type Key struct {
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	PrivateKey []byte    `json:"-"` // raw bytes, never serialized to JSON
	CreatedAt  time.Time `json:"created_at"`
}

// storedKey is the on-disk JSON format for an encrypted wallet.
type storedKey struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	EncryptedKey string `json:"encrypted_key"`
	CreatedAt    string `json:"created_at"`
}

// Keystore manages encrypted wallet storage in a directory.
type Keystore struct {
	dir string
}

// TODO: Replace with passphrase-derived key via scrypt when golang.org/x/crypto
// is added. This fixed dev key is for development/testing only.
var devEncryptionKey = sha256.Sum256([]byte("peth-dev-encryption-key"))

// NewKeystore creates a new Keystore backed by the given directory.
// It ensures the directory exists.
func NewKeystore(dir string) (*Keystore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create keystore dir: %w", err)
	}
	return &Keystore{dir: dir}, nil
}

// Create generates a new ECDSA key, derives an Ethereum-style address,
// encrypts the private key, and saves it to disk.
func (ks *Keystore) Create(name string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	// TODO: Replace P-256 with secp256k1 when go-ethereum is available.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	return ks.saveKey(name, privKey)
}

// Import imports a wallet from a hex private key or a BIP39 mnemonic phrase.
// It auto-detects the format: hex keys are 64 hex chars (optionally 0x-prefixed),
// mnemonics are space-separated words.
func (ks *Keystore) Import(name string, input string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	input = strings.TrimSpace(input)

	// Detect format: hex key vs mnemonic
	if isHexKey(input) {
		return ks.importHex(name, input)
	}
	if isMnemonic(input) {
		return ks.importMnemonic(name, input)
	}

	return nil, fmt.Errorf("unrecognized key format: must be hex private key or BIP39 mnemonic")
}

// List returns all wallets in the keystore (without private keys).
func (ks *Keystore) List() ([]*Key, error) {
	matches, err := filepath.Glob(filepath.Join(ks.dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list wallets: %w", err)
	}

	keys := make([]*Key, 0, len(matches))
	for _, path := range matches {
		sk, err := ks.readStored(path)
		if err != nil {
			continue // skip corrupt files
		}
		t, _ := time.Parse(time.RFC3339, sk.CreatedAt)
		keys = append(keys, &Key{
			Name:      sk.Name,
			Address:   sk.Address,
			CreatedAt: t,
			// PrivateKey intentionally omitted
		})
	}
	return keys, nil
}

// Get retrieves a specific wallet by name (with decrypted private key).
func (ks *Keystore) Get(name string) (*Key, error) {
	path := ks.path(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrWalletNotFound
	}

	sk, err := ks.readStored(path)
	if err != nil {
		return nil, err
	}

	privBytes, err := decrypt(sk.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt key: %w", err)
	}

	t, _ := time.Parse(time.RFC3339, sk.CreatedAt)
	return &Key{
		Name:       sk.Name,
		Address:    sk.Address,
		PrivateKey: privBytes,
		CreatedAt:  t,
	}, nil
}

// Delete removes a wallet from the keystore.
func (ks *Keystore) Delete(name string) error {
	path := ks.path(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrWalletNotFound
	}
	return os.Remove(path)
}

// Use sets the active wallet by writing the name to a .active file.
func (ks *Keystore) Use(name string) error {
	if !ks.exists(name) {
		return ErrWalletNotFound
	}
	activePath := filepath.Join(ks.dir, ".active")
	return os.WriteFile(activePath, []byte(name), 0600)
}

// Active returns the currently active wallet.
func (ks *Keystore) Active() (*Key, error) {
	activePath := filepath.Join(ks.dir, ".active")
	data, err := os.ReadFile(activePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no active wallet set")
		}
		return nil, err
	}
	return ks.Get(strings.TrimSpace(string(data)))
}

// --- internal helpers ---

func (ks *Keystore) exists(name string) bool {
	_, err := os.Stat(ks.path(name))
	return err == nil
}

func (ks *Keystore) path(name string) string {
	return filepath.Join(ks.dir, name+".json")
}

func (ks *Keystore) readStored(path string) (*storedKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sk storedKey
	if err := json.Unmarshal(data, &sk); err != nil {
		return nil, fmt.Errorf("parse wallet file: %w", err)
	}
	return &sk, nil
}

func (ks *Keystore) saveKey(name string, privKey *ecdsa.PrivateKey) (*Key, error) {
	privBytes := privKey.D.Bytes()
	// Pad to 32 bytes if needed
	if len(privBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(privBytes):], privBytes)
		privBytes = padded
	}

	addr := deriveAddress(privKey)

	encKey, err := encrypt(privBytes)
	if err != nil {
		return nil, fmt.Errorf("encrypt key: %w", err)
	}

	now := time.Now().UTC()
	sk := storedKey{
		Name:         name,
		Address:      addr,
		EncryptedKey: encKey,
		CreatedAt:    now.Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(sk, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(ks.path(name), data, 0600); err != nil {
		return nil, err
	}

	return &Key{
		Name:       name,
		Address:    addr,
		PrivateKey: privBytes,
		CreatedAt:  now,
	}, nil
}

func (ks *Keystore) importHex(name string, input string) (*Key, error) {
	input = strings.TrimPrefix(input, "0x")
	privBytes, err := hex.DecodeString(input)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}

	// TODO: Replace P-256 with secp256k1 when go-ethereum is available.
	curve := elliptic.P256()
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privBytes)

	return ks.saveKey(name, privKey)
}

func (ks *Keystore) importMnemonic(name string, mnemonic string) (*Key, error) {
	// TODO: Implement full BIP39/BIP44 derivation when go-ethereum is available.
	// For now, derive a deterministic key from the mnemonic using SHA-256
	// as a placeholder. This produces a valid key structure but NOT a
	// BIP39-compliant derivation path.
	seed := sha256.Sum256([]byte(mnemonic))

	// TODO: Replace P-256 with secp256k1 when go-ethereum is available.
	curve := elliptic.P256()
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(seed[:])
	// Ensure D is within the curve order
	privKey.D.Mod(privKey.D, curve.Params().N)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privKey.D.Bytes())

	return ks.saveKey(name, privKey)
}

// deriveAddress computes an Ethereum-style address from an ECDSA public key.
// TODO: Replace SHA-256 with Keccak-256 when go-ethereum is available.
// Real Ethereum: keccak256(pubKeyBytes)[12:32] → 0x-prefixed hex.
func deriveAddress(privKey *ecdsa.PrivateKey) string {
	pubBytes := elliptic.Marshal(privKey.PublicKey.Curve, privKey.PublicKey.X, privKey.PublicKey.Y)
	// Skip the 0x04 prefix byte (uncompressed point marker)
	hash := sha256.Sum256(pubBytes[1:])
	// Take last 20 bytes as address
	addr := hash[12:]
	return "0x" + hex.EncodeToString(addr)
}

// isHexKey returns true if the input looks like a hex-encoded private key.
func isHexKey(s string) bool {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// isMnemonic returns true if the input looks like a BIP39 mnemonic (12 or 24 words).
func isMnemonic(s string) bool {
	words := strings.Fields(s)
	return len(words) == 12 || len(words) == 24
}

// encrypt encrypts plaintext bytes with AES-256-GCM using the dev key.
func encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(devEncryptionKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext using the dev key.
func decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(devEncryptionKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
