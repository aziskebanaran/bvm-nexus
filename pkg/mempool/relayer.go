package mempool

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

func StartRelayer(m *NexusMempool, coreURL string) {
    fmt.Println("🚀 [RELAYER] Sentinel Sequencer Aktif...")

    for {
        time.Sleep(3 * time.Second) // Batching setiap 3 detik

        txs := m.Flush()
        if len(txs) == 0 { continue }

        jsonData, _ := json.Marshal(txs)
        resp, err := http.Post(coreURL+"/api/mempool/batch", "application/json", bytes.NewBuffer(jsonData))

        if err != nil || resp.StatusCode != http.StatusOK {
            fmt.Printf("⚠️ [RELAYER] Core Sibuk. Mengembalikan %d tx ke antrean...\n", len(txs))
            // Masukkan kembali ke mempool agar tidak hilang
            for _, tx := range txs {
                m.AddL2(tx) 
            }
            continue
        }
        
        fmt.Printf("✅ [RELAYER] %d Transaksi L2 berhasil dipahat ke Core L1.\n", len(txs))
        resp.Body.Close()
    }
}
