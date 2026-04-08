package secrets

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	storeDir = ".store"
	saltSize = 16

	// SecretsFile is the filename for the encrypted secrets store.
	SecretsFile = "secrets.enc"
)

// SecretsPath returns the full path to the secrets file given a repo root.
func SecretsPath(root string) string {
	return filepath.Join(root, storeDir, SecretsFile)
}

// Exists returns true if the encrypted secrets file exists.
func Exists(root string) bool {
	_, err := os.Stat(SecretsPath(root))
	return err == nil
}

// Load decrypts the secrets file and returns all secrets as a map.
// Returns an empty map if the file doesn't exist yet.
func Load(root string, passphrase string) (map[string]string, error) {
	data, err := os.ReadFile(SecretsPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read secrets file: %w", err)
	}

	plaintext, err := decrypt(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets file: wrong passphrase or corrupted data")
	}

	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, fmt.Errorf("decode secrets file: %w", err)
	}
	if secrets == nil {
		return map[string]string{}, nil
	}

	return secrets, nil
}

// Save encrypts the secrets map and writes it to the secrets file.
// Creates the .store/ directory if needed.
func Save(root string, passphrase string, secrets map[string]string) error {
	if secrets == nil {
		secrets = map[string]string{}
	}

	plaintext, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	data, err := encrypt(plaintext, passphrase)
	if err != nil {
		return fmt.Errorf("encrypt secrets: %w", err)
	}

	path := SecretsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write secrets file: %w", err)
	}

	return nil
}

func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, 1, 64*1024, 4, 32)
}

func encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := deriveKey(passphrase, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	data := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	data = append(data, salt...)
	data = append(data, nonce...)
	data = append(data, ciphertext...)

	return data, nil
}

func decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) < saltSize+chacha20poly1305.NonceSizeX {
		return nil, fmt.Errorf("encrypted secrets data is too short")
	}

	salt := data[:saltSize]
	nonceStart := saltSize
	nonceEnd := nonceStart + chacha20poly1305.NonceSizeX
	nonce := data[nonceStart:nonceEnd]
	ciphertext := data[nonceEnd:]

	key := deriveKey(passphrase, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("authenticate secrets data: %w", err)
	}

	return plaintext, nil
}
