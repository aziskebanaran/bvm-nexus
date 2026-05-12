package state

import (
    "fmt"
    "time"

    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    coretypes "github.com/aziskebanaran/bvm-core/x/bvm/types"
    wasmkeeper "github.com/aziskebanaran/bvm-core/x/wasm/keeper"
    "github.com/aziskebanaran/bvm-lib/constants"
    "github.com/aziskebanaran/bvm-lib/game"
    "github.com/aziskebanaran/bvm-lib/utils"
    "github.com/aziskebanaran/bvm-nexus/pkg/snapshot"
    "github.com/vmihailenco/msgpack/v5"
    "github.com/aziskebanaran/bvm-nexus/pkg/vm"
    "github.com/aziskebanaran/bvm-lib/sdk"
)


const MarketAddress = "bvmf_market_system_vault"

func StartBatchSync(c *client.BVMClient, store storage.BVMStore, wk *wasmkeeper.Keeper, vmEngine *vm.WASMEngine) {

    fmt.Println("🚀 [NEXUS] Memulai Sinkronisasi Global...")

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

				var finalBlock coretypes.Block
				tmp, _ := msgpack.Marshal(sdkBlock)
				msgpack.Unmarshal(tmp, &finalBlock)

				// 🚩 1. OPERASI SENTINEL: Cek Konstitusi Era Modern (WASM Guard)
				if h >= 6000 && wk != nil {
					if err := wk.ValidateBlock(finalBlock); err != nil {
						fmt.Printf("❌ [SENTINEL] Blok #%d DITOLAK: %v\n", h, err)
						return // Hentikan sinkronisasi jika blok tidak sah
					}
				}

				// 🚩 2. SAVE BLOCK (Fungsi Vital yang Sempat Hilang)
				// Kita simpan menggunakan SaveBlock agar indeks internal core storage jalan
				store.SaveBlock(finalBlock) 
				// Kita tambahkan juga mapping manual menggunakan BuildCoreKey agar API HandleGetBlock sinkron
				store.Put(utils.BuildCoreKey(constants.DBBlockPrefix, fmt.Sprint(h)), finalBlock)

				// 🚩 3. LOGIKA EKONOMI & REWARD (Setiap 10 Blok)
				if h%10 == 0 {
					// --- A. GACHA MARKET ---
					asset := MintGachaAsset(int64(h), finalBlock.Hash)
					store.Put(utils.BuildNexusKey(constants.PrefixMarket, asset.ID), asset)
					fmt.Printf("🎁 [GACHA] %s tersedia di rak!\n", asset.Name)

					// --- B. BONUS VALIDATOR (Nexus Loyalty) ---
					vAddress := finalBlock.Miner
					var ethAddr string
					// Cek mapping identitas (id:)
					errMap := store.Get(utils.BuildNexusKey(constants.PrefixIdentity, vAddress), &ethAddr)
					if errMap == nil && ethAddr != "" {
						reward := game.Item{
							ID:    fmt.Sprintf("REWARD-%d-%s", h, vAddress[:6]),
							Name:  "Validator Loyalty Crate",
							Type:  "chest",
							Power: 50,
						}
						store.Put(utils.BuildSubKey(constants.PrefixInv, ethAddr, reward.ID), reward)
						fmt.Printf("🛰️ [REWARD] Crate dikirim ke %s\n", ethAddr[:10])
					}
				}

				// 🚩 4. OPERASI CRAWLER & INDEXING (Indexing Transaksi)
				for _, tx := range finalBlock.Transactions {

					// --- Update History (Prefix h:) ---
					for _, addr := range []string{tx.From, tx.To} {
						var history []coretypes.Transaction
						histKey := utils.BuildNexusKey(constants.PrefixHistory, addr)
						store.Get(histKey, &history)

						exists := false
						for _, old := range history {
							if old.ID == tx.ID { exists = true; break }
						}
						if !exists {
							history = append([]coretypes.Transaction{tx}, history...)
							if len(history) > 50 { history = history[:50] }
							store.Put(histKey, history)
						}
					}

					// --- Sinkronisasi Saldo & Nonce (Prefix s: dan n:) ---
					if acc, err := c.GetAccount(tx.From); err == nil {
						store.Put(utils.BuildNexusKey(constants.PrefixState, tx.From), acc.Balances["BVM"])
						store.Put(utils.BuildCoreKey(constants.DBNoncePrefix, tx.From), acc.Nonce)
					}

					// --- Indexing Transaksi (Prefix t:) ---
					store.Put(utils.BuildCoreKey(constants.DBTxPrefix, tx.ID), tx)


    // --- [B] OTOMATISASI UTXO L2 (SOLUSI SALDO) ---
    // Pemicu agar saldo tidak lagi 'null' saat di-curl
    if tx.Amount > 0 {
        // Gunakan 16 karakter pertama TXID agar kepingan punya ID unik
        utxoID := fmt.Sprintf("%s-0", tx.ID[:16]) 

        // Membangun kunci database sesuai standar Jenderal: utxo:[ALAMAT]:[TXID-0]
        dbKey := utils.BuildSubKey(constants.PrefixUTXO, tx.To, utxoID)

        // Simpan jumlah koin dalam bentuk string unit atomik agar mudah dibaca oleh Store
        store.PutRaw([]byte(dbKey), []byte(fmt.Sprintf("%d", tx.Amount)))

        fmt.Printf("💎 [SYNC-UTXO] Kepingan dicetak otomatis untuk %s | Nilai: %d\n", tx.To[:10], tx.Amount)
    }

    // --- [C] EKSEKUSI NATIVE VM (Jika ke Market) ---
    if tx.To == MarketAddress {
        ctx := sdk.Context{
            Sender:       tx.From,
            ContractAddr: MarketAddress,
            BlockHeight:  int64(h),
            TxHash:       tx.ID,
            Timestamp:    time.Now().Unix(),
        }

        // Menjalankan PURCHASE_ASSET yang sudah kita perbaiki tadi
        _, err := vmEngine.ExecuteNative("PURCHASE_ASSET", ctx, []byte(tx.Memo))
        if err != nil {
            fmt.Printf("⚠️ [SYNC-VM] Gagal: %v\n", err)
        }
    }
}


				// Update Progres
				store.Put("latest_height", h)
				MonitorArchive(int64(h))

				if h%10 == 0 {
					fmt.Printf("🔗 [ANCHOR] Blok #%d diverifikasi.\n", h)
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
}


// ... (MonitorArchive & MintGachaAsset tetap sama) ...

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


// MintGachaAsset: Fungsi internal untuk menentukan huruf/angka
func MintGachaAsset(height int64, blockHash string) *game.Item {
    characters := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    
    // Ambil benih acak dari karakter pertama hash blok
    seed := int(blockHash[0])
    if len(blockHash) > 1 {
        seed += int(blockHash[1]) // Tambah variasi
    }
    
    selectedChar := string(characters[seed % len(characters)])

    return &game.Item{
        ID:        fmt.Sprintf("ASSET-%d", height),
        Name:      fmt.Sprintf("Karakter [%s]", selectedChar),
        Type:      "material",
        Power:     1,
        Stackable: true,
    }
}

func SyncValidators(c *client.BVMClient, store storage.BVMStore) {
    // Logika sinkronisasi tanpa menggunakan struct 'n'
    info, err := c.GetNetworkInfo()
    if err == nil {
        fmt.Printf("📡 [SYNC] Validator Info synced at height %d\n", info.Height)
    }
}

