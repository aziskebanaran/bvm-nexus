package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	// Menggunakan bvm-lib yang sudah lulus sensor
	"github.com/aziskebanaran/bvm-lib/constants"
	"github.com/aziskebanaran/bvm-lib/crypto"
	"github.com/aziskebanaran/bvm-lib/utils"
        "github.com/aziskebanaran/bvm-core/x/bvm/types"
         core_types "github.com/aziskebanaran/bvm-lib/core-types"
)

func (h *NexusHandler) HandleCreateWallet(w http.ResponseWriter, r *http.Request) {
	// 1. Buat Wallet menggunakan bvm-lib/crypto
	// Sekarang CreateFromMnemonic sudah menghasilkan 3 identitas sekaligus!
	mnemonic, _ := crypto.GenerateMnemonic()
	newW, err := crypto.CreateFromMnemonic(mnemonic, 0)
	if err != nil {
		http.Error(w, "Gagal cetak identitas kriptografi", 500)
		return
	}

	// 2. REGISTRASI MAPPING (Gunakan KeyBuilder & Constants)
	// Memetakan ETH Style (0x) ke BVM Native
	h.Store.Put(utils.BuildKey(constants.PrefixIdentity, newW.EthAddress), newW.Address)
	
	// Memetakan Nexus ID ke BVM Native
	h.Store.Put(utils.BuildKey(constants.PrefixIdentity, newW.NexusAddress), newW.Address)
	
	// Back-reference: Alamat BVM ke ETH (untuk reverse lookup)
	h.Store.Put(utils.BuildKey(constants.PrefixIdentity, newW.Address), newW.EthAddress)

	// 3. REGISTRASI ALIAS (Username)
	username := r.URL.Query().Get("username")
	if username != "" {
		if !strings.HasPrefix(username, "@") {
			username = "@" + username
		}
		h.Store.Put(utils.BuildKey(constants.PrefixIdentity, username), newW.Address)
		fmt.Printf("🏷️ [NEXUS-IDENTITY] Alias Terdaftar: %s\n", username)
	}

	fmt.Printf("🎭 [NEXUS-IDENTITY] Multi-Mapping Berhasil: %s\n", newW.Address[:10])

	// 4. RESPON DATA LENGKAP (Struktur bvm-lib/crypto.BVMWallet)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newW)
}

// HandleResolve menerjemahkan Username, 0x, atau Nexus ID menjadi Alamat BVM Native
func (h *NexusHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", 400)
		return
	}

	var targetAddress string
	// Gunakan utils.BuildKey untuk keamanan query
	searchKey := utils.BuildKey(constants.PrefixIdentity, query)
	
	// 1. Cari di Database Mapping Nexus
	err := h.Store.Get(searchKey, &targetAddress)

	identityType := "DIRECT_ADDRESS"
	if err == nil && targetAddress != "" {
		// Deteksi tipe untuk log
		if strings.HasPrefix(query, "@") {
			identityType = "USERNAME"
		} else if strings.HasPrefix(query, "0x") {
			identityType = "ETH_STYLE"
		} else if strings.HasPrefix(query, "nexus_") {
			identityType = "NEXUS_ID"
		} else {
			identityType = "MAPPED_BVM"
		}
		fmt.Printf("🎯 [RESOLVER] %s -> %s (%s)\n", query, targetAddress[:10], identityType)
	} else {
		// Fallback: Jika tidak ketemu, anggap input adalah alamat tujuan langsung
		targetAddress = query
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"address":       targetAddress,
		"resolved_from": query,
		"type":          identityType,
	})
}

func (h *NexusHandler) BuildWalletMetadata(address string) core_types.WalletState {
    // 1. Standarisasi Alamat (Resolusi bvmf vs 0x)
    finalBVMAddr := address
    var mappedETH string

    if strings.HasPrefix(address, "0x") {
        // Jika input 0x, cari alamat aslinya (bvmf) di database identitas
        h.Store.Get("id:"+address, &finalBVMAddr)
        mappedETH = address
    } else if strings.HasPrefix(address, "bvmf") {
        // Jika input bvmf, cari apakah sudah dipetakan ke 0x
        h.Store.Get("id:"+address, &mappedETH)
    }

    state := core_types.WalletState{
        Address:       finalBVMAddr, // Menjamin "Alamat Asli" tetap bvmf
        MappedAddress: mappedETH,    // Menampilkan 0x sebagai info tambahan
        Symbol:        "BVM",
    }

    // 2. Akses Cache & Prosedur Force Sync (Fokus pada finalBVMAddr)
    type StateCache struct { Balance uint64; Nonce uint64 }
    var cache StateCache
    
    // Gunakan BuildNexusKey agar sinkron dengan pkg/state/sync.go
    h.Store.Get(utils.BuildNexusKey(constants.PrefixState, finalBVMAddr), &cache)

    // 🚩 FORCE SYNC: Menjamin laporan tidak 0.00 jika Core punya saldo
    if cache.Balance == 0 {
        targetURL := fmt.Sprintf("%s/api/balance?address=%s", h.CoreURL, finalBVMAddr)
        resp, err := http.Get(targetURL)
        if err == nil {
            defer resp.Body.Close()
            var rawData map[string]interface{}
            if errD := json.NewDecoder(resp.Body).Decode(&rawData); errD == nil {
                cache.Balance = convertToUint64(rawData["balance_atomic"])
                cache.Nonce = convertToUint64(rawData["nonce"])
                // Simpan ke Cache Nexus (L2) untuk efisiensi ke depan
                h.Store.Put(utils.BuildNexusKey(constants.PrefixState, finalBVMAddr), cache)
            }
        }
    }

    state.BalanceAtomic = cache.Balance
    state.Nonce = cache.Nonce
    state.BalanceDisplay = fmt.Sprintf("%.8f", float64(cache.Balance)/100000000.0)

    // 3. Konsolidasi Data Aset (UTXO & History)
    var utxoIDs []string
    h.Store.Get("index:utxo:"+finalBVMAddr, &utxoIDs)
    state.UTXOCount = len(utxoIDs)

    var history []types.Transaction
    h.Store.Get(utils.BuildNexusKey(constants.PrefixHistory, finalBVMAddr), &history)
    state.ActivityCount = len(history)
    
    if len(history) > 0 {
        state.LastSeen = history[0].Timestamp
        state.LastMemo = history[0].Memo
    }

    return state
}

func (h *NexusHandler) DetermineTier(txCount int) string {
    if txCount > 100 { return "Merchant" }
    if txCount > 20 { return "Pro" }
    return "Basic"
}
