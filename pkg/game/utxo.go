package game

import (
    "crypto/sha256"
    "fmt"
    "time"
    "github.com/aziskebanaran/bvm-nexus/pkg/utxo" // Standar Unifikasi
)

// CreateGameUTXO: Mengubah blueprint dari generator menjadi kepingan L2
func CreateGameUTXO(txHash string, owner string, assetID string, name string) utxo.UTXO {
    return utxo.UTXO{
        TxID:      txHash,
        Index:     0,
        Address:   owner,
        Amount:    0,
        AssetID:   assetID,
        Metadata:  name,
        Type:      "GAME_ITEM",
        Status:    "UNSPENT",
        Timestamp: time.Now().Unix(),
    }
}

// GenerateUTXOID: Serial number unik untuk kepingan tersebut
func GenerateUTXOID(txHash string, index int) string {
    data := fmt.Sprintf("%s-%d-%d", txHash, index, time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash)[:16]
}
