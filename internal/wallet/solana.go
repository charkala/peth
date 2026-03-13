package wallet

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// base58 alphabet (Bitcoin/Solana standard)
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58Encode encodes bytes to a base58 string.
func Base58Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Count leading zeros
	leadingZeros := 0
	for _, b := range data {
		if b != 0 {
			break
		}
		leadingZeros++
	}

	// Convert to big integer
	n := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var encoded []byte
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		encoded = append([]byte{base58Alphabet[mod.Int64()]}, encoded...)
	}

	// Add leading '1's for each leading zero byte
	for i := 0; i < leadingZeros; i++ {
		encoded = append([]byte{'1'}, encoded...)
	}

	return string(encoded)
}

// Base58Decode decodes a base58 string to bytes.
func Base58Decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("empty base58 string")
	}

	// Build lookup table
	lookup := make(map[byte]int64)
	for i, c := range base58Alphabet {
		lookup[byte(c)] = int64(i)
	}

	// Count leading '1's
	leadingOnes := 0
	for _, c := range s {
		if c != '1' {
			break
		}
		leadingOnes++
	}

	// Convert from base58
	n := new(big.Int)
	base := big.NewInt(58)
	for i := 0; i < len(s); i++ {
		val, ok := lookup[s[i]]
		if !ok {
			return nil, fmt.Errorf("invalid base58 character: %c", s[i])
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(val))
	}

	decoded := n.Bytes()
	// Add leading zero bytes
	result := make([]byte, leadingOnes+len(decoded))
	copy(result[leadingOnes:], decoded)

	return result, nil
}

// solanaStoredKey is the on-disk JSON format for an encrypted Solana wallet.
type solanaStoredKey struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	EncryptedKey string `json:"encrypted_key"`
	CreatedAt    string `json:"created_at"`
	KeyType      string `json:"key_type"` // always "ed25519"
}

// SolanaKeystore manages encrypted Solana wallet storage.
type SolanaKeystore struct {
	dir string
}

// NewSolanaKeystore creates a new SolanaKeystore backed by the given directory.
func NewSolanaKeystore(dir string) (*SolanaKeystore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create solana keystore dir: %w", err)
	}
	return &SolanaKeystore{dir: dir}, nil
}

// Create generates a new ed25519 keypair and saves it as a Solana wallet.
func (ks *SolanaKeystore) Create(name string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	address := Base58Encode(pub)
	return ks.saveKey(name, address, priv)
}

// Import imports a Solana wallet from a base58-encoded private key.
func (ks *SolanaKeystore) Import(name string, input string) (*Key, error) {
	if ks.exists(name) {
		return nil, ErrWalletExists
	}

	privBytes, err := Base58Decode(input)
	if err != nil {
		return nil, fmt.Errorf("decode base58 key: %w", err)
	}

	// ed25519 private keys are 64 bytes (seed + public key)
	priv := ed25519.PrivateKey(privBytes)
	pub := priv.Public().(ed25519.PublicKey)
	address := Base58Encode(pub)

	return ks.saveKey(name, address, priv)
}

// List returns all Solana wallets in the keystore (without private keys).
func (ks *SolanaKeystore) List() ([]*Key, error) {
	matches, err := filepath.Glob(filepath.Join(ks.dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list solana wallets: %w", err)
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

// Get retrieves a specific Solana wallet by name (with decrypted private key).
func (ks *SolanaKeystore) Get(name string) (*Key, error) {
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

// --- internal helpers ---

func (ks *SolanaKeystore) exists(name string) bool {
	_, err := os.Stat(ks.path(name))
	return err == nil
}

func (ks *SolanaKeystore) path(name string) string {
	return filepath.Join(ks.dir, name+".json")
}

func (ks *SolanaKeystore) readStored(path string) (*solanaStoredKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sk solanaStoredKey
	if err := json.Unmarshal(data, &sk); err != nil {
		return nil, fmt.Errorf("parse solana wallet file: %w", err)
	}
	return &sk, nil
}

func (ks *SolanaKeystore) saveKey(name string, address string, privKey ed25519.PrivateKey) (*Key, error) {
	encKey, err := encrypt([]byte(privKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt key: %w", err)
	}

	now := time.Now().UTC()
	sk := solanaStoredKey{
		Name:         name,
		Address:      address,
		EncryptedKey: encKey,
		CreatedAt:    now.Format(time.RFC3339),
		KeyType:      "ed25519",
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
		Address:    address,
		PrivateKey: []byte(privKey),
		CreatedAt:  now,
	}, nil
}
