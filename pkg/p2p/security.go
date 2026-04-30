package p2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt" // Ditambahkan untuk fmt.Printf
	"io"
	"time" // Ditambahkan untuk time.Now()

	"github.com/libp2p/go-libp2p/core/crypto" // Ditambahkan untuk tipe PubKey
)

// EncryptMessage membungkus data sensitif Sultan (AES-GCM)
func EncryptMessage(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptMessage membuka bungkusan pesan rahasia (AES-GCM)
func DecryptMessage(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext terlalu pendek")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, actualCiphertext, nil)
}

// VerifyIdentity memastikan bahwa pengirim pesan benar-benar pemilik NodeID tersebut
func VerifyIdentity(nodeID string, signature []byte, timestamp int64, pubKey crypto.PubKey) bool {
	// 1. Cek apakah pesan sudah kadaluarsa (mencegah Replay Attack)
	if time.Now().Unix()-timestamp > 60 {
		fmt.Println("⚠️ [SECURITY] Pesan kadaluarsa!")
		return false
	}

	// 2. Susun kembali pesan aslinya sesuai protokol jabat tangan
	msg := fmt.Sprintf("%s:%d", nodeID, timestamp)

	// 3. Verifikasi tanda tangan menggunakan Kunci Publik pengirim
	isValid, err := pubKey.Verify([]byte(msg), signature)
	if err != nil || !isValid {
		fmt.Printf("❌ [SECURITY] Identitas Palsu terdeteksi dari Node: %s\n", nodeID)
		return false
	}

	return true
}
