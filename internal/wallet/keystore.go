// Package wallet provides encrypted wallet key management for peth.
package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
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

// devEncryptionKey is used for AES-256-GCM encryption of stored keys.
// TODO: Replace with passphrase-derived key via scrypt when desired.
var devEncryptionKey = sha256.Sum256([]byte("peth-dev-encryption-key"))

// NewKeystore creates a new Keystore backed by the given directory.
func NewKeystore(dir string) (*Keystore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create keystore dir: %w", err)
	}
	return &Keystore{dir: dir}, nil
}

// Create generates a new secp256k1 private key, derives a real Ethereum address,
// encrypts the private key, and saves it to disk.
func (ks *Keystore) Create(name string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	return ks.saveKey(name, privKey.Serialize())
}

// Import imports a wallet from a hex private key or a BIP39 mnemonic phrase.
func (ks *Keystore) Import(name string, input string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	input = strings.TrimSpace(input)

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
			continue
		}
		t, _ := time.Parse(time.RFC3339, sk.CreatedAt)
		keys = append(keys, &Key{
			Name:      sk.Name,
			Address:   sk.Address,
			CreatedAt: t,
		})
	}
	return keys, nil
}

// Get retrieves a wallet by name (with decrypted private key).
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

// PersonalSign implements eth_personal_sign.
// Hashes the message with the Ethereum prefix and signs with secp256k1.
// Returns a 0x-prefixed 65-byte signature: [r (32)] [s (32)] [v (1)].
// v is 27 or 28 per Ethereum convention.
func PersonalSign(privKeyBytes []byte, message string) (string, error) {
	hash := PersonalSignHash([]byte(message))
	sig, err := signHash(privKeyBytes, hash)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(sig), nil
}

// PersonalSignHash computes the Ethereum personal_sign prefix hash:
// keccak256("\x19Ethereum Signed Message:\n" + len(message) + message)
func PersonalSignHash(message []byte) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(prefix))
	h.Write(message)
	return h.Sum(nil)
}

// signHash produces a recoverable secp256k1 ECDSA signature (65 bytes).
// Uses ecdsa.SignCompact which returns [recovery_flag (1)] [r (32)] [s (32)].
// We convert to Ethereum format: [r (32)] [s (32)] [v (1)] where v = 27 + recovery_flag.
func signHash(privKeyBytes []byte, hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}

	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)

	// SignCompact returns: [flag (1 byte)] [r (32 bytes)] [s (32 bytes)]
	// flag = 27 + recovery_id (+ 4 if compressed pubkey)
	compact := ecdsa.SignCompact(privKey, hash, false) // false = uncompressed pubkey
	if len(compact) != 65 {
		return nil, fmt.Errorf("unexpected compact signature length: %d", len(compact))
	}

	// Convert decred compact format to Ethereum format
	// decred: [flag][r][s] where flag = 27 + recid (uncompressed)
	// ethereum: [r][s][v] where v = 27 + recid
	flag := compact[0]
	r := compact[1:33]
	s := compact[33:65]

	// v in Ethereum format: flag is already 27+recid for uncompressed
	v := flag

	out := make([]byte, 65)
	copy(out[0:32], r)
	copy(out[32:64], s)
	out[64] = v
	return out, nil
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

func (ks *Keystore) saveKey(name string, privBytes []byte) (*Key, error) {
	addr, err := deriveAddress(privBytes)
	if err != nil {
		return nil, fmt.Errorf("derive address: %w", err)
	}

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
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(privBytes))
	}
	return ks.saveKey(name, privBytes)
}

func (ks *Keystore) importMnemonic(name string, mnemonic string) (*Key, error) {
	// TODO: Implement full BIP39/BIP44 derivation.
	// For now, derive a deterministic key from the mnemonic using SHA-256.
	seed := sha256.Sum256([]byte(mnemonic))
	return ks.saveKey(name, seed[:])
}

// deriveAddress computes a real Ethereum address from secp256k1 private key bytes.
// Ethereum address = keccak256(uncompressed_pubkey[1:])[12:]
func deriveAddress(privBytes []byte) (string, error) {
	privKey := secp256k1.PrivKeyFromBytes(privBytes)
	pubKey := privKey.PubKey()

	// Uncompressed public key: 0x04 || X (32 bytes) || Y (32 bytes) = 65 bytes
	pubBytes := pubKey.SerializeUncompressed()

	// keccak256 of pubKey bytes (skip the leading 0x04 uncompressed marker)
	h := sha3.NewLegacyKeccak256()
	h.Write(pubBytes[1:])
	hash := h.Sum(nil)

	// Ethereum address = last 20 bytes of keccak256(pubkey)
	return "0x" + hex.EncodeToString(hash[12:]), nil
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

// encrypt encrypts plaintext bytes with AES-256-GCM.
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

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
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
