package state

import (
    "fmt"
    "time"
    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/x/bvm/types"
    "github.com/aziskebanaran/bvm-core/x/wasm/keeper"
    "github.com/aziskebanaran/bvm-nexus/pkg/snapshot"
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


		// 1. Simpan Blok (Wajib)
		store.SaveBlock(finalBlock)


                // 🚩 2. OPERASI CRAWLER & INDEXING (Edisi Sentinel)
                for _, tx := range finalBlock.Transactions {
                    fmt.Printf("📦 [CRAWLER] Memproses Transaksi: %s\n", tx.ID[:8])

                    // A. Update History Pengirim (From)
                    var historyFrom []types.Transaction
                    store.Get("h:"+tx.From, &historyFrom)
                    existsFrom := false
                    for _, old := range historyFrom { if old.ID == tx.ID { existsFrom = true; break } }
                    if !existsFrom {
                        historyFrom = append([]types.Transaction{tx}, historyFrom...)
                        store.Put("h:"+tx.From, historyFrom)
                    }

                    // B. Update History Penerima (To)
                    var historyTo []types.Transaction
                    store.Get("h:"+tx.To, &historyTo)
                    existsTo := false
                    for _, old := range historyTo { if old.ID == tx.ID { existsTo = true; break } }
                    if !existsTo {
                        historyTo = append([]types.Transaction{tx}, historyTo...)
                        store.Put("h:"+tx.To, historyTo)
                    }

                    // C. Sinkronisasi Saldo & Nonce (Tetap Dipertahankan)
                    if acc, err := c.GetAccount(tx.From); err == nil {
                        // Gunakan balance_atomic jika ada, atau map Balances
                        bal := acc.Balances["BVM"]
                        store.Put("s:"+tx.From, bal)
                        store.Put("n:"+tx.From, acc.Nonce)
                    }
                    if acc, err := c.GetAccount(tx.To); err == nil {
                        bal := acc.Balances["BVM"]
                        store.Put("s:"+tx.To, bal)
                        store.Put("n:"+tx.To, acc.Nonce)
                    }

                    // D. Simpan Detail Transaksi untuk /api/tx
                    store.Put("t:"+tx.ID, tx)
                    store.Put("tx:"+tx.ID, tx) // Double prefix agar aman
                }


                // Update tinggi blok lokal
                store.Put("latest_height", h)
                store.Put("m:height", h)

                // 🚩 UNTUK AUTO-ARCHIVE (Tambahkan baris ini)
                MonitorArchive(int64(h)) 

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


// MonitorArchive memantau tinggi blok dan memicu pembuatan snapshot
func MonitorArchive(currentHeight int64) {
    // Tentukan interval, misal setiap 1000 blok
    const ArchiveInterval = 1000

    if currentHeight > 0 && currentHeight % ArchiveInterval == 0 {
        fmt.Printf("📦 [AUTO-ARCHIVE] Mencapai tinggi % d. Menyiapkan snapshot...\n", currentHeight)
        
        src := "./data_nexus/blockchain_db"
        dest := "./data_nexus/blockchain_db/snapshot.tar.gz"
        
        err := snapshot.CreateSnapshot(src, dest)
        if err != nil {
            fmt.Printf("⚠️ [AUTO-ARCHIVE] Gagal membuat snapshot: %v\n", err)
        } else {
            fmt.Println("✅ [AUTO-ARCHIVE] Snapshot terbaru siap didistribusikan!")
        }
    }
}
