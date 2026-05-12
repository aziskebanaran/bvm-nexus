package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"crypto/sha256"
	"encoding/hex"

	"github.com/aziskebanaran/bvm-lib/constants"
	"github.com/aziskebanaran/bvm-lib/utils"
	"github.com/aziskebanaran/bvm-nexus/pkg/utxo"
	nexusgame "github.com/aziskebanaran/bvm-nexus/pkg/game"
	"github.com/aziskebanaran/bvm-core/x/bvm/types"
)

// Helper internal jika utils.GenerateRandomHash tidak ditemukan
func generateTxID() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h[:])
}

func (h *NexusHandler) HandleAutoSpend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Amount uint64 `json:"amount"`
		Fee    uint64 `json:"fee"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Instruksi Cacat", 400)
		return
	}

	// 1. CARI KEPINGAN (Gunakan h.UTXOStore yang sudah kita daftarkan)
	inputs, totalValue, err := h.UTXOStore.SpendUTXO(req.From, req.Amount+req.Fee)
	if err != nil {
		http.Error(w, "Saldo Kepingan Tidak Cukup: "+err.Error(), 400)
		return
	}

	// 2. PROSES TRANSFER & KEMBALIAN
	txID := generateTxID() // 🚩 Gunakan helper internal agar aman

	// Cetak Kepingan untuk Penerima
	h.MintNewUTXO(req.To, txID, 0, req.Amount)

	// 🚩 CETAK KEMBALIAN (Change Address)
	if totalValue > (req.Amount + req.Fee) {
		change := totalValue - (req.Amount + req.Fee)
		h.MintNewUTXO(req.From, txID, 1, change)
	}

	// 3. HAPUS KEPINGAN LAMA
	for _, in := range inputs {
		h.UTXOStore.RemoveUTXO(req.From, in.PrevTxID, in.Index)
	}

	// 4. LAPOR KE CORE L1
	l1Tx := types.Transaction{
		From:   req.From,
		To:     req.To,
		Amount: req.Amount,
		Memo:   "UTXO_L2_CONSOLIDATED",
	}
	// Pastikan h.Mempool sudah diinisialisasi di NexusHandler
	h.Mempool.AddL2(l1Tx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "SUCCESS",
		"txid_l2":         txID,
		"spent_inputs":    len(inputs),
		"change_returned": totalValue - (req.Amount + req.Fee),
	})
}

func (h *NexusHandler) MintNewUTXO(addr string, txID string, index int, amount uint64) {
    newU := utxo.UTXO{
        TxID:      txID,
        Index:     index,
        Address:   addr,
        Amount:    amount,
        Status:    "UNSPENT", // 🚩 Tambahkan status agar terbaca validator
        Type:      "COIN",    // 🚩 Tambahkan tipe
        Timestamp: time.Now().Unix(),
    }
    h.UTXOStore.SaveUTXO(newU)
}


func (h *NexusHandler) MintUTXOFromPurchase(userAddr string, txID string, metadata string) {
        // 🚩 Ganti nexusgame.UTXO menjadi utxo.UTXO
        newUTXO := utxo.UTXO{
                AssetID:   nexusgame.GenerateUTXOID(txID, 0), // Tetap pakai generator ID unik
                TxID:      txID,
                Index:     0,
                Address:   userAddr,
                Status:    "UNSPENT",
                Metadata:  metadata,
                Type:      "GAME_ITEM", // Tandai sebagai item game
                Timestamp: time.Now().Unix(),
        }

        // Simpan ke UTXOStore agar terdeteksi sistem koin & inventaris
        h.UTXOStore.SaveUTXO(newUTXO)
        fmt.Printf("💎 [UTXO] Kepingan dicetak: %s\n", newUTXO.AssetID[:8])
}

func (h *NexusHandler) HandleGetUTXOs(w http.ResponseWriter, r *http.Request) {
        userAddr := r.URL.Query().Get("address")

        // Ambil kepingan menggunakan UTXOStore yang sudah terstandarisasi
        results, err := h.UTXOStore.GetUTXOsByAddress(userAddr)
        if err != nil {
                http.Error(w, "Gagal akses gudang", 500)
                return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(results)
}

func (h *NexusHandler) UpdateUTXOIndex(from, to, id string) {
    // Hapus dari daftar Pengirim
    var fromList []string
    h.Store.Get("index:utxo:"+from, &fromList)
    newList := []string{}
     for _, item := range fromList {
        if item != id { newList = append(newList, item) }
    }
    h.Store.Put("index:utxo:"+from, newList)

    // Tambah ke daftar Penerima
    var toList []string
    h.Store.Get("index:utxo:"+to, &toList)
    toList = append(toList, id)
    h.Store.Put("index:utxo:"+to, toList)
}

func (h *NexusHandler) HandleTransferUTXO(w http.ResponseWriter, r *http.Request) {
    var req struct {
        UTXOID string `json:"utxo_id"` 
        From   string `json:"from"`    
        To     string `json:"to"`      
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Instruksi Cacat", 400)
        return
    }

    resolvedTo := h.ResolveToBVM(req.To)
    resolvedFrom := h.ResolveToBVM(req.From)

    var targetUTXO utxo.UTXO

    // 🚩 PERBAIKAN: Gunakan konstanta global untuk log agar tidak membingungkan
    fullPrefix := fmt.Sprintf("%s%s:", constants.PrefixUTXO, resolvedFrom)

    // Ambil kepingan menggunakan UTXOStore (Sudah otomatis pakai prefix yang benar)
    utxos, err := h.UTXOStore.GetUTXOsByAddress(resolvedFrom)
    if err != nil {
        http.Error(w, "Gagal akses gudang UTXO", 500)
        return
    }

    found := false
    for _, u := range utxos {
        // 🚩 LOGIKA MATCHING: Cocokkan ID Aset atau TxID
        if u.AssetID == req.UTXOID || u.TxID == req.UTXOID {
            targetUTXO = u
            found = true
            break
        }
    }

    if !found {
        fmt.Printf("⚠️ [TRANSFER-FAIL] Aset %s tidak ditemukan di %s\n", req.UTXOID, fullPrefix)
        http.Error(w, "🚫 Kepingan aset tidak ditemukan!", 403)
        return
    }

    // 3. EKSEKUSI (Atomic Swap)
    h.UTXOStore.RemoveUTXO(resolvedFrom, targetUTXO.TxID, targetUTXO.Index)

    targetUTXO.Address = resolvedTo
    targetUTXO.Timestamp = time.Now().Unix()

    h.UTXOStore.SaveUTXO(targetUTXO)

    // 4. UPDATE INDEKS
    h.UpdateUTXOIndex(resolvedFrom, resolvedTo, req.UTXOID)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "SUCCESS",
        "msg": "Aset Game berhasil dipindahkan!",
    })
}

// Helper untuk resolusi alamat di Nexus
func (h *NexusHandler) ResolveToBVM(query string) string {
    var addr string
    h.Store.Get(utils.BuildKey(constants.PrefixIdentity, query), &addr)
    if addr == "" { return query }
    return addr
}

func (h *NexusHandler) HandleContractCall(w http.ResponseWriter, r *http.Request) {
    // 1. Tangkap parameter dari request
    // 2. Siapkan sdk.Context (Sender, BlockHeight, dll)
    // 3. Panggil WASMEngine untuk mengeksekusi node_manager.wasm
    
    fmt.Println("📡 [RPC-CONTRACT] Menerima instruksi eksekusi WASM...")
}
