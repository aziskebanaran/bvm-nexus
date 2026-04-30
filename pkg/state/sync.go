package state

import (
    "fmt"
    "time"
    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/x/bvm/types"
    "github.com/aziskebanaran/bvm-core/x/wasm/keeper"
    "github.com/vmihailenco/msgpack/v5"
)

func StartBatchSync(c *client.BVMClient, store storage.BVMStore, wk *keeper.Keeper) {
    fmt.Println("🚀 [NEXUS] Memulai Sinkronisasi Global (Sentinel Mode)...")

    for {
        info, err := c.GetNetworkInfo()
        if err != nil {
            fmt.Println("⚠️ Core Lokal Offline, mencari data di database Nexus...")
            time.Sleep(10 * time.Second)
            continue
        }

        localHeight := GetLocalHeight(store)

        if uint64(info.Height) > localHeight {
            targetHeight := uint64(info.Height)

            for h := localHeight + 1; h <= targetHeight; h++ {
                sdkBlock, err := c.GetBlockByHeight(h)
                if err != nil { 
                    fmt.Printf("❌ Gagal ambil blok #%d\n", h)
                    break 
                }

                // Konversi data menggunakan MsgPack agar kompatibel
                var finalBlock types.Block
                tmp, _ := msgpack.Marshal(sdkBlock)
                msgpack.Unmarshal(tmp, &finalBlock)

                // 🚩 OPERASI SENTINEL: Cek Konstitusi Era Modern
                if h >= 6000 && wk != nil {
                    // Validasi State & Transaksi menggunakan WASM
                    if err := wk.ValidateBlock(finalBlock); err != nil {
                        fmt.Printf("❌ [SENTINEL] Blok #%d DITOLAK: %v\n", h, err)
                        // Kita berhenti sinkronisasi jika blok tidak sah untuk keamanan
                        return 
                    }
                }

                // 1. Simpan ke LevelDB Lokal Nexus jika lolos verifikasi
                if err := store.SaveBlock(finalBlock); err != nil {
                    fmt.Printf("❌ Gagal simpan blok #%d: %v\n", h, err)
                    break
                }
                // Update tinggi blok lokal
		store.Put("latest_height", h)
                store.Put("m:height", h)

                // 2. LOGIKA MONITORING
                if h % 10 == 0 {
                    fmt.Printf("🔗 [ANCHOR] Blok #%d diverifikasi & diarsipkan.\n", h)
                }

                if h % 50 == 0 || h == targetHeight {
                    fmt.Printf("🧱 [SYNC] Progres Nexus: %d / %d | Health: SENTINEL_OK\n", h, targetHeight)
                }
            }
        }
        time.Sleep(2 * time.Second)
    }
}

func GetLocalHeight(store storage.BVMStore) uint64 {
    var h uint64
    
    // 1. Coba ambil dari key baru
    err := store.Get("latest_height", &h)
    
    // 2. Jika key baru kosong (h == 0), ambil dari cadangan (key lama)
    if err != nil || h == 0 {
        _ = store.Get("m:height", &h)
    }
    
    return h
}
