package snapshot

import (
    "fmt"
    "net/http"
)

// 🚩 Pastikan H-nya KAPITAL agar bisa dipanggil dari luar
func HandleDownloadSnapshot(w http.ResponseWriter, r *http.Request) {
    fmt.Println("📦 Seseorang sedang menarik snapshot dari Nexus Sultan...")
    w.Header().Set("Content-Disposition", "attachment; filename=blockchain_snapshot.db")
    http.ServeFile(w, r, "./data_nexus/blockchain_db/snapshot.tar.gz")
}
