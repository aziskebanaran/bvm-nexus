package snapshot

import (
    "fmt"
    "net/http"
	"os"
)

func HandleDownloadSnapshot(w http.ResponseWriter, r *http.Request) {
    snapshotPath := "./data_nexus/blockchain_db/snapshot.tar.gz"
    dbPath := "./data_nexus/blockchain_db"

    // Jika file belum ada, kita buatkan dulu (atau bisa dibuat terjadwal)
    if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
        fmt.Println("📦 Snapshot tidak ditemukan. Membuat snapshot baru...")
        // Peringatan: Idealnya DB ditutup sementara atau gunakan Snapshot LevelDB
        CreateSnapshot(dbPath, snapshotPath)
    }

    fmt.Println("🚀 Mengirim snapshot ke peer...")
    http.ServeFile(w, r, snapshotPath)
}
