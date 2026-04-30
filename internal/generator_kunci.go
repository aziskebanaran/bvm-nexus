package main

import (
    "crypto/ed25519"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
)

type NexusIdentity struct {
    PeerID    string `json:"peer_id"`
    SecretKey string `json:"secret_key"`
}

func main() {
    // 1. Generate Key Pair Standar ED25519
    pub, priv, err := ed25519.GenerateKey(nil)
    if err != nil {
        fmt.Println("❌ Gagal membuat identitas:", err)
        return
    }

    // 2. Format murni untuk BVM Nexus
    peerID := hex.EncodeToString(pub)
    secretKey := hex.EncodeToString(priv)

    fmt.Println("🌐 BVM NEXUS - IDENTITY GENERATOR")
    fmt.Println("==========================================")
    fmt.Printf("✅ PEER ID   : %s\n", peerID)
    fmt.Println("==========================================")

    // Simpan ke nexus_identity.json (Bukan near_key lagi)
    identity := NexusIdentity{
        PeerID:    peerID,
        SecretKey: secretKey,
    }
    
    data, _ := json.MarshalIndent(identity, "", "  ")
    os.WriteFile("internal/nexus_identity.json", data, 0600)
    
    fmt.Println("📝 Identitas disimpan ke: internal/nexus_identity.json")
}
