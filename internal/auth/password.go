package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argonParams holds the tuning parameters for argon2id password hashing.
// These values are encoded into the hash string so VerifyPassword can decode
// them later, which makes future parameter upgrades possible.
type argonParams struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultArgonParams = argonParams{
	memory:      64 * 1024,
	iterations:  1,
	parallelism: 4,
	saltLength:  16,
	keyLength:   32,
}

// HashPassword hashes a plaintext password using argon2id.
//
// We return an encoded string containing the parameters, salt, and hash.
// Storing parameters alongside the hash means verification still works if we
// change defaults for new passwords later.
func HashPassword(password string) (string, error) {
	p := defaultArgonParams
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// PHC-style encoding: $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.iterations, p.parallelism, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword checks a plaintext password against an encoded argon2id hash.
// It returns (false, nil) when the password does not match, and an error only
// for malformed hashes/system failures.
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, hash, err := decodeArgonHash(encoded)
	if err != nil {
		return false, err
	}

	other := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)
	// Constant-time comparison avoids leaking timing information about the hash.
	if subtle.ConstantTimeCompare(hash, other) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeArgonHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return argonParams{}, nil, nil, fmt.Errorf("invalid hash format")
	}
	if parts[1] != "argon2id" {
		return argonParams{}, nil, nil, fmt.Errorf("unsupported hash algorithm: %s", parts[1])
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return argonParams{}, nil, nil, fmt.Errorf("unsupported argon2 version: %s", parts[2])
	}

	var p argonParams
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return argonParams{}, nil, nil, fmt.Errorf("invalid hash parameter: %s", kv)
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return argonParams{}, nil, nil, fmt.Errorf("invalid hash parameter value: %s", kv)
		}
		switch k {
		case "m":
			p.memory = uint32(n)
		case "t":
			p.iterations = uint32(n)
		case "p":
			p.parallelism = uint8(n)
		default:
			return argonParams{}, nil, nil, fmt.Errorf("unknown hash parameter: %s", k)
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	p.saltLength = uint32(len(salt))
	p.keyLength = uint32(len(hash))
	return p, salt, hash, nil
}
