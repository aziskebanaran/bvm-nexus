package user

import (
    "encoding/json"
    "net/http"
    "strings"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-lib/game"
)

// HandleGetInventory: Mengambil isi tas pemain berdasarkan ETH Address (Standar Go)
func HandleGetInventory(store storage.BVMStore, w http.ResponseWriter, r *http.Request) {
    // 1. Ambil alamat dari URL (Misal: /api/inventory/0x123...)
    // Kita potong path-nya
    parts := strings.Split(r.URL.Path, "/")
    if len(parts) < 4 {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": "Address required"})
        return
    }
    ethAddr := parts[3]

    // 2. Ambil data dari database Nexus
    var inv game.Inventory
    err := store.Get("inv:state:"+ethAddr, &inv)

    if err != nil {
        // Inisialisasi tas baru jika belum ada
        inv = game.Inventory{
            OwnerAddress: ethAddr,
            Capacity:     20,
            Slots:        []game.Slot{},
        }
    }

    // 3. Kirim JSON ke Jenderal
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "data":   inv,
    })
}
