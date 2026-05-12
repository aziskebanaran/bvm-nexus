package utxo

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
)

// ValidateOwnership memeriksa apakah signature pada input sah menggunakan standar ECDSA ASN1 Jenderal
func ValidateOwnership(txID string, signatureHex string, pubKeyHex string) error {
	// 1. Decode Public Key dari Hex
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("public key korup")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubBytes)
	if err != nil {
		return fmt.Errorf("gagal parse public key")
	}

	ecdsaPubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("bukan kunci ECDSA yang sah")
	}

	// 2. Decode Signature dari Hex
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("signature korup")
	}

	// 3. Hitung Hash dari txID (Pastikan ini sesuai dengan CalculateHash di Core)
	hash := sha256.Sum256([]byte(txID))

	// 4. Verifikasi menggunakan standar ASN1 (Sesuai verifyECDSAModern di AuthKeeper)
	isValid := ecdsa.VerifyASN1(ecdsaPubKey, hash[:], sigBytes)

	if !isValid {
		return fmt.Errorf("🚨 Pelanggaran! Tanda tangan PALSU untuk kepingan ini")
	}
	return nil
}
