package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aziskebanaran/bvm-lib/constants"
	"github.com/aziskebanaran/bvm-lib/game"
	"github.com/aziskebanaran/bvm-lib/utils"
	pkgtypes "github.com/aziskebanaran/bvm-core/pkg/types"
	nexusgame "github.com/aziskebanaran/bvm-nexus/pkg/game"
)

func (h *NexusHandler) HandleMarketList(w http.ResponseWriter, r *http.Request) {
	items := make(map[string]interface{})
	currentHeight := h.GetLocalHeight()
	startScan := (currentHeight / 10) * 10

	for i := 0; i < 5; i++ {
		targetH := startScan - uint64(i*10)
		if targetH <= 0 { break }

		assetID := fmt.Sprintf("ASSET-%d", targetH)
		key := utils.BuildNexusKey(constants.PrefixMarket, assetID)
		
		var asset game.Item
		if err := h.Store.Get(key, &asset); err == nil && asset.Name != "" {
			items[assetID] = map[string]interface{}{
				"id":            assetID,
				"name":          asset.Name,
				"power":         asset.Power,
				"price_display": "0.01000000 BVM",
				"price_atomic":  pkgtypes.BVM_UNIT / 100,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "OPEN",
		"items":  items,
		"height": currentHeight,
	})
}

func (h *NexusHandler) HandleMarketBuy(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("id")
	userAddr := r.URL.Query().Get("from")

	if assetID == "" || userAddr == "" {
		http.Error(w, "❌ Data tidak lengkap", 400)
		return
	}

	keyMarket := utils.BuildNexusKey(constants.PrefixMarket, assetID)
	var item game.Item
	if err := h.Store.Get(keyMarket, &item); err != nil {
		http.Error(w, "Aset tidak ditemukan", 404)
		return
	}

	// Simulasi Pembayaran ke Core
	nonce, _ := h.CoreClient.GetNextNonce(userAddr)
	tx := pkgtypes.Transaction{
		From: userAddr, To: "bvmf_market_vault",
		Amount: pkgtypes.BVM_UNIT / 100, Nonce: nonce,
	}
	txID, err := h.CoreClient.BroadcastTX(tx)
	if err != nil {
		http.Error(w, "🚨 Gagal Bayar: "+err.Error(), 500)
		return
	}

	h.TransferAssetToUser(userAddr, assetID, item)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "SUCCESS", "tx_id": txID})
}


func (h *NexusHandler) TransferAssetToUser(userAddr string, assetID string, item game.Item) {
    // 1. RESOLUSI ALAMAT (Pastikan selalu bvmf)
    finalAddr := h.ResolveToBVM(userAddr)

    // 2. SIMPAN SEBAGAI UTXO CRYPTO ASSET (Agar terdeteksi PrefixScan di Inventory)
    // Gunakan prefix "utxo:" agar sinkron dengan HandleGetInventory Langkah 3-B
    utxoKey := fmt.Sprintf("utxo:%s:%s", finalAddr, assetID)
    
    utxoData := map[string]interface{}{
        "id":       assetID,
        "metadata": item.Name,
        "status":   "UNSPENT",
        "type":     "UTXO_CRYPTO_ASSET",
        "power":    item.Power,
    }
    
    h.Store.Put(utxoKey, utxoData)

    // 3. DAFTARKAN KE INDEKS (Untuk fungsi pencarian cepat)
    var ownedIDs []string
    h.Store.Get("user:assets:"+finalAddr, &ownedIDs)
    
    // Cek agar tidak duplikat
    exists := false
    for _, id := range ownedIDs {
        if id == assetID { exists = true; break }
    }
    if !exists {
        ownedIDs = append(ownedIDs, assetID)
        h.Store.Put("user:assets:"+finalAddr, ownedIDs)
    }

    // 4. HAPUS DARI ETALASE MARKET
    keyMarket := utils.BuildNexusKey(constants.PrefixMarket, assetID)
    h.Store.Delete(keyMarket)
    
    fmt.Printf("📦 [MARKET-SUCCESS] Aset %s dipindahkan ke gudang UTXO: %s\n", assetID, finalAddr[:10])
}


func (h *NexusHandler) HandleGetInventory(w http.ResponseWriter, r *http.Request) {
    // --- 🚩 LANGKAH 0: AMBIL ALAMAT DARI PATH (Sesuai parts[3]) ---
    parts := strings.Split(r.URL.Path, "/")
    var inputAddr string
    if len(parts) >= 4 {
        inputAddr = parts[3]
    }

    if inputAddr == "" {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "ERROR",
            "message": "Address is required",
        })
        return
    }

    inventory := make(map[string]interface{})

    // --- 🚩 LANGKAH 1: RESOLUSI IDENTITAS (Username -> Address) ---
    finalAddr := inputAddr
    if !strings.HasPrefix(inputAddr, "bvmf") && !strings.HasPrefix(inputAddr, "0x") {
        var resolvedAddr string
        h.Store.Get("user:"+inputAddr, &resolvedAddr)
        if resolvedAddr != "" {
            finalAddr = resolvedAddr
        }
    }

    // --- 🚩 LANGKAH 2: IDENTIFIKASI IDENTITAS GANDA ---
    var bvmAddr string = finalAddr
    var ethAddr string

    if strings.HasPrefix(finalAddr, "0x") {
        ethAddr = finalAddr
        h.Store.Get("id:"+ethAddr, &bvmAddr)
    } else if strings.HasPrefix(finalAddr, "bvmf") {
        bvmAddr = finalAddr
        h.Store.Get("id:"+bvmAddr, &ethAddr)
    }

    // --- 🚩 LANGKAH 3: SCAN GUDANG (Direct Prefix Hunter) ---
    scanGudang := func(targetAddr string) {
        if targetAddr == "" { return }

        // A. Item Game Legacy
        var ownedIDs []string
        h.Store.Get("user:assets:"+targetAddr, &ownedIDs)
        for _, id := range ownedIDs {
            var item game.Item
            if err := h.Store.Get("inv:"+targetAddr+":"+id, &item); err == nil && item.ID != "" {
                inventory[id] = item
            }
        }

        // B. UTXO Crypto Asset (Menggunakan prefix yang kita tanam di sync.go)
        prefix := "utxo:" + targetAddr + ":"
        results, _ := h.Store.PrefixScan(prefix)

        for _, v := range results {
            // Karena di sync.go Jenderal menyimpan pakai map[string]interface{},
            // kita gunakan unmarshal map agar lebih fleksibel
            var u map[string]interface{}
            if err := json.Unmarshal(v, &u); err == nil {
                id, _ := u["id"].(string)
                if id != "" {
                    inventory[id] = map[string]interface{}{
                        "id":     id,
                        "name":   u["metadata"],
                        "type":   "UTXO_CRYPTO_ASSET",
                        "status": u["status"],
                    }
                }
            }
        }
    }

    // Eksekusi pemindaian dua arah
    scanGudang(bvmAddr)
    if ethAddr != "" && ethAddr != bvmAddr {
        scanGudang(ethAddr)
    }

    // --- 🚩 LANGKAH 4: RESPON JSON (Kunci Sinkronisasi Wallet) ---
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "SUCCESS",
        "owner":  bvmAddr,
        "items":  inventory, // Nama field harus "items" sesuai bvm-wallet
        "count":  len(inventory),
    })
}


func (h *NexusHandler) HandleRepairInventory(w http.ResponseWriter, r *http.Request) {
    userAddr := r.URL.Query().Get("address")
    finalAddr := h.ResolveToBVM(userAddr)

    fmt.Printf("🛠️ [REPAIR-INTEL] Menjemput aset untuk: %s\n", finalAddr[:10])

    // 1. Tembak langsung ke Core API (Gunakan CoreURL dari config)
    targetURL := fmt.Sprintf("%s/api/history?address=%s", h.CoreURL, finalAddr)
    resp, err := http.Get(targetURL)
    if err != nil {
        http.Error(w, "Core L1 Tidak Terjangkau: "+err.Error(), 500)
        return
    }
    defer resp.Body.Close()

    // 2. Decode Struktur JSON History yang Jenderal tunjukkan tadi
    var coreRes struct {
        History []struct {
            ID   string `json:"id"`
            Memo string `json:"memo"`
        } `json:"history"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&coreRes); err != nil {
        http.Error(w, "Format Riwayat Core Berubah/Rusak", 500)
        return
    }

    count := 0
    for _, tx := range coreRes.History {
        // 3. Deteksi Pembelian yang belum tercatat di L2
        if strings.Contains(tx.Memo, "Purchase:") {
            assetID := strings.TrimPrefix(tx.Memo, "Purchase: ")

	    utxoID := nexusgame.GenerateUTXOID(tx.ID, 0)

            // Cek apakah sudah ada di gudang UTXO Nexus
	    utxoKey := fmt.Sprintf("utxo:%s:%s", finalAddr, utxoID)
            var existing map[string]interface{}
            
            if err := h.Store.Get(utxoKey, &existing); err != nil {
                // 🚩 BELUM ADA: Paksa Cetak ke Gudang Jenderal!
                dummyItem := game.Item{Name: "Karakter [O] (Recovered)", Power: 100}
                h.TransferAssetToUser(finalAddr, assetID, dummyItem)
                count++
                fmt.Printf("✅ [RECOVERED] %s berhasil diklaim kembali!\n", assetID)
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "SUCCESS",
        "recovered_count": count,
        "message": "Gudang Jenderal telah disinkronkan dengan Core L1",
    })
}
