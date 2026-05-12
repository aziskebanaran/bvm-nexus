package game

import (
    "fmt"
    "github.com/aziskebanaran/bvm-lib/game" // Menggunakan library Jenderal
)

const MarketAddress = "bvmf_market_system_vault"

// MintGachaAsset: Menghasilkan aset berdasarkan DNA Blok
func MintGachaAsset(height int64, blockHash string) *game.Item {
    if height % 10 != 0 { return nil }

    // 1. Gunakan Hash Blok sebagai Seed
    characters := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    
    // Ambil byte pertama dari hash untuk menentukan index
    seed := int(blockHash[0]) 
    assetChar := string(characters[seed % len(characters)])

    // 2. Bungkus menjadi Item bvm-lib
    return &game.Item{
        ID:        fmt.Sprintf("ASSET-%d", height),
        Name:      fmt.Sprintf("Karakter [%s]", assetChar),
        Type:      "material",
        Power:     1,
        Stackable: true,
    }
}

func GetValidatorBonusItem(height int64, bvmAddr string) *game.Item {
    return &game.Item{
        ID:    fmt.Sprintf("REWARD-%d-%s", height, bvmAddr[:6]),
        Name:  "Validator Loyalty Crate",
        Type:  "chest",
        Power: 50,
    }
}
