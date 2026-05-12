package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aziskebanaran/bvm-core/x/bvm/types"
	"github.com/aziskebanaran/bvm-lib/constants" // 🚩 Doktrin Global
	"github.com/aziskebanaran/bvm-lib/utils"     // 🚩 Mesin Kunci
)

// Gunakan PrefixVault agar alamat sistem ini terstandarisasi
var DexVaultAddr = utils.BuildKey(constants.PrefixVault, "reserve_system")

func (h *NexusHandler) HandleSwap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string `json:"from"`
		Amount uint64 `json:"amount"`
		Target string `json:"target_asset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", 400)
		return
	}

	// 1. Validasi via BuildWalletMetadata
	meta := h.BuildWalletMetadata(req.From)
	if meta.BalanceAtomic < req.Amount {
		http.Error(w, "Saldo Tidak Cukup untuk Swap", 400)
		return
	}

	// 2. Eksekusi Perpindahan Saldo L2 (Gunakan UpdateL2Balance yang sudah standar)
	h.UpdateL2Balance(req.From, -int64(req.Amount))
	h.UpdateL2Balance(DexVaultAddr, int64(req.Amount))

	// 3. Masukkan ke Mempool untuk Laporan Relayer ke Core L1
	tx := types.Transaction{
		From:      req.From,
		To:        DexVaultAddr,
		Amount:    req.Amount,
		Timestamp: time.Now().Unix(),
		Memo:      "DEX_SWAP:" + req.Target,
	}
	h.Mempool.AddL2(tx) // Menggunakan AddL2 yang sudah pintar resolusi

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "SUCCESS",
		"vault":  DexVaultAddr,
		"txid":   tx.ID,
	})
}

func (h *NexusHandler) HandleAddLiquidity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string `json:"from"`
		Amount uint64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Request Cacat", 400)
		return
	}

	meta := h.BuildWalletMetadata(req.From)
	if meta.BalanceAtomic < req.Amount {
		http.Error(w, "Saldo Sultan tidak cukup untuk menyumbang", 400)
		return
	}

	tx := types.Transaction{
		From:      req.From,
		To:        DexVaultAddr,
		Amount:    req.Amount,
		Timestamp: time.Now().Unix(),
		Memo:      "ADD_LIQUIDITY_L2",
	}

	h.Mempool.AddL2(tx)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "LIQUIDITY_ADDED",
		"msg":    "Terima kasih atas kontribusinya, Jenderal!",
		"txid":   tx.ID,
	})
}

func (h *NexusHandler) HandleRemoveLiquidity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To        string `json:"to"`
		Amount    uint64 `json:"amount"`
		Signature string `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload Cacat", 400)
		return
	}

	vaultMeta := h.BuildWalletMetadata(DexVaultAddr)
	if vaultMeta.BalanceAtomic < req.Amount {
		http.Error(w, "Likuiditas Brankas Tidak Mencukupi", 400)
		return
	}

	tx := types.Transaction{
		From:      DexVaultAddr,
		To:        req.To,
		Amount:    req.Amount,
		Timestamp: time.Now().Unix(),
		Memo:      "WITHDRAW_PROFIT_L2",
	}

	success := h.Mempool.AddL2(tx)

	if success {
		fmt.Printf("💰 [VAULT] Profit sebesar %d Atomic berhasil ditarik ke %s\n", req.Amount, req.To)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "SUCCESS",
			"msg":    "Profit berhasil diamankan ke dompet Sultan",
			"txid":   tx.ID,
		})
	}
}

// UpdateL2Balance patuh sepenuhnya pada bvm-lib
func (h *NexusHandler) UpdateL2Balance(addr string, change int64) {
	type StateCache struct {
		Balance uint64 `json:"balance"`
		Nonce   uint64 `json:"nonce"`
	}
	var cache StateCache

	// 🛡️ Gunakan PrefixState ("s:") dari bvm-lib
	// BuildNexusKey menjamin kunci bersih tanpa duplikasi prefix
	key := utils.BuildNexusKey(constants.PrefixState, addr)

	h.Store.Get(key, &cache)

	if change > 0 {
		cache.Balance += uint64(change)
	} else {
		absChange := uint64(-change)
		if cache.Balance >= absChange {
			cache.Balance -= absChange
		} else {
			cache.Balance = 0 // Proteksi underflow
		}
	}

	h.Store.Put(key, cache)
}
