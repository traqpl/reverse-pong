package certdata

//go:generate go run ../../cmd/certgen

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

// _masks are XOR obfuscation values; _kp[i] ^ _masks[i] = actual AES key part i.
// Must match masks in cmd/certgen/main.go.
var _masks = [4]uint64{
	0x48A5C3D2F1E6B790,
	0x2C7E91F4A836D05B,
	0xF3821C0ED7594A6E,
	0x6B4F2A8DE1097C35,
}

func aesKey() []byte {
	key := make([]byte, 32)
	for i, p := range _kp {
		binary.LittleEndian.PutUint64(key[i*8:], p^_masks[i])
	}
	return key
}

// Load decrypts and returns the certificate chain PEM and private key PEM.
func Load() (certChainPEM, keyPEM []byte, err error) {
	key := aesKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("certdata: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("certdata: %w", err)
	}
	certChainPEM, err = gcm.Open(nil, _nonce1[:], _encCert, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("certdata: decrypt cert: %w", err)
	}
	keyPEM, err = gcm.Open(nil, _nonce2[:], _encKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("certdata: decrypt key: %w", err)
	}
	return
}
