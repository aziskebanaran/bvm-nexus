package rpc

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

// HandleBridgeMint memproses pencetakan saldo yang dilaporkan oleh bvm-bridge
func (h *NexusHandler) HandleBridgeMint(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        Address    string `json:"address"`
        Amount     uint64 `json:"amount"`
        ExternalTX string `json:"external_tx"`
        Source     string `json:"source"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Bad Payload", http.StatusBadRequest)
        return
    }

    // 🚩 PERBAIKAN KRITIS 1: Pagar Keamanan String Slicing
    // Mencegah runtime error: slice bounds out of range
    if len(req.ExternalTX) < 8 {
        fmt.Printf("⚠️  [BRIDGE] Request ditolak: ExternalTX '%s' terlalu pendek.\n", req.ExternalTX)
        http.Error(w, "ExternalTX must be at least 8 characters", http.StatusBadRequest)
        return
    }

    // 1. CEK DATABASE (Idempotency) 
    // Menggunakan Full TX agar benar-benar unik di database
    key := "bridge_processed:" + req.ExternalTX
    var alreadyProcessed bool
    err := h.Store.Get(key, &alreadyProcessed)

    if err == nil && alreadyProcessed {
        fmt.Printf("⚠️  [NEXUS] Transaksi %s sudah pernah diproses! Menolak duplikasi.\n", req.ExternalTX[:8])
        w.WriteHeader(http.StatusConflict)
        json.NewEncoder(w).Encode(map[string]string{"error": "Double mint detected"})
        return
    }

    // 2. EKSEKUSI: Ambil 8 karakter dengan aman untuk Memo
    shortTX := req.ExternalTX[:8] 
    const BridgeVault = "bvmf_bridge_vault_address"

    // Membangun instruksi pelepasan aset dari Core Vault
    payload := map[string]interface{}{
        "from":   BridgeVault,
        "to":     req.Address,
        "amount": req.Amount,
        "memo":   "BRIDGE_RELEASE:" + shortTX,
    }

    jsonData, _ := json.Marshal(payload)
    
    // Kirim perintah ke Core (Port 8080)
    resp, err := http.Post(h.CoreURL+"/api/send", "application/json", bytes.NewBuffer(jsonData))

    if err != nil || resp.StatusCode != http.StatusOK {
        fmt.Printf("❌ [BRIDGE] Core menolak pelepasan aset: %v\n", err)
        http.Error(w, "Vault Release Failed", http.StatusInternalServerError)
        return
    }

    // 3. ARCHIVING: Simpan status agar transaksi ini tidak bisa diulang
    err = h.Store.Put(key, true)
    if err != nil {
        fmt.Printf("❌ [BRIDGE] Gagal mencatat riwayat: %v\n", err)
        // Kita tetap lanjut karena transfer di Core sudah sukses
    } else {
        fmt.Printf("💾 [BRIDGE] Transaksi %s berhasil diarsipkan.\n", shortTX)
    }

    // Berikan balasan sukses ke client
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "MINT_SUCCESS",
        "tx_ref": req.ExternalTX,
    })
}
