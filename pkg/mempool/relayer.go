package mempool

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// StartRelayer akan berjalan di background untuk mengirim TX ke Core
func StartRelayer(m *NexusMempool, coreURL string) {
    fmt.Println("🚀 [RELAYER] Mempool Relayer aktif. Siap menyetor transaksi ke Core...")
    
    for {
        time.Sleep(2 * time.Second) // Setoran setiap 2 detik

        txs := m.Flush()
        if len(txs) == 0 {
            continue
        }

        // Kirim batch transaksi ke endpoint mempool Core
        jsonData, _ := json.Marshal(txs)
        resp, err := http.Post(coreURL+"/api/mempool/batch", "application/json", bytes.NewBuffer(jsonData))
        
        if err != nil {
            fmt.Printf("⚠️ [RELAYER] Gagal setoran: Core Offline (%v)\n", err)
            // Opsional: Jika gagal, masukkan kembali txs ke mempool (Retry)
            continue
        }
        
        if resp.StatusCode == http.StatusOK {
            fmt.Printf("✅ [RELAYER] Berhasil menyetor %d transaksi ke Core.\n", len(txs))
        }
        resp.Body.Close()
    }
}
