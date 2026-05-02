package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/pkg/p2p"
    "github.com/aziskebanaran/bvm-core/x/wasm/keeper" // 🚩 Import WasmKeeper Core

    nexusP2P "github.com/aziskebanaran/bvm-nexus/pkg/p2p"
    "github.com/aziskebanaran/bvm-nexus/pkg/rpc"
    "github.com/aziskebanaran/bvm-nexus/pkg/state"
    "github.com/aziskebanaran/bvm-nexus/pkg/snapshot"
    "github.com/ipfs/go-log/v2"
)

func main() {
    log.SetAllLoggers(log.LevelFatal)
    fmt.Println("🌐 BVM-NEXUS: Modular Mode Activated")

    // 1. Inisialisasi Database Nexus
    nexusStore, err := storage.NewLevelDBStore("./data_nexus/blockchain_db", 8)
    if err != nil {
        panic(err)
    }
    defer nexusStore.Close()

    // 🚩 2. Inisialisasi SENTINEL (Wasm Keeper)
    // Sesuai keeper.go di Core: func NewKeeper(store storage.BVMStore) *Keeper
    wk := keeper.NewKeeper(nexusStore) 

    // Opsional: Jika Jenderal ingin melacak jalur wasm manual
    fmt.Println("🧠 [SENTINEL] Unit Keeper berhasil diaktifkan di Nexus.")

    // 3. Jalankan P2P TCP Listener (Core)
    go p2p.StartNode(9091)

    // 4. Inisialisasi Client ke Core Utama (Port 8080)
    coreAddr := "http://localhost:8080"
    c := client.NewBVMClient(coreAddr)

    // 🚩 5. Jalankan Background Sync (Kirim 'wk' ke unit State)
    // Sekarang unit State bisa memanggil wk.ValidateBlock(block)
    go state.StartBatchSync(c, nexusStore, wk)

    // 6. Inisialisasi Global DHT (Global Radar)
    ctx := context.Background()
    dhtHost, _, err := nexusP2P.StartGlobalDHT(ctx, 9093)
    if err != nil {
        fmt.Printf("⚠️ Gagal menyalakan DHT: %v\n", err)
    } else {
        defer dhtHost.Close()
    }

    // 7. Auto-Bootstrap & Cleanup
    go nexusP2P.AutoBootstrap(":9092")
    go func() {
        fmt.Println("🧹 [SYSTEM] Petugas pembersihan peer aktif.")
        for {
            nexusP2P.CleanInactivePeers()
            time.Sleep(24 * time.Hour)
        }
    }()


    // 8. API Handler Setup (Koordinat Perubahan Jenderal di Sini)
    handler := &rpc.NexusHandler{
        Store:      nexusStore,
        CoreURL:    coreAddr,
        CoreClient: c, // 🚩 PASANG UNIT CLIENT DI SINI (menggunakan variabel 'c' yang sudah dibuat di atas)
    }

// --- REGISTRASI ROUTE (EDISI GABUNGAN TERKUAT) ---

// 1. RUTE SPESIFIK (Dahulukan rute yang butuh penanganan khusus)
http.HandleFunc("/api/discover-core", handler.MinerDiscoveryHandler)
http.HandleFunc("/api/login", handler.HandleProxyAuth)
http.HandleFunc("/api/nexus/status", handler.HandleNexusStatus)
http.HandleFunc("/api/nexus/update-peers", handler.HandleUpdatePeers)
http.HandleFunc("/peers", handler.HandleGetPeers)
http.HandleFunc("/p2p/register", handler.HandleRegisterPeer)
http.HandleFunc("/api/balance", handler.HandleBalance)

// 1. RUTE SPESIFIK
http.HandleFunc("/api/snapshot/download", snapshot.HandleDownloadSnapshot)
http.HandleFunc("/api/wallet/create", handler.HandleCreateWallet)
http.HandleFunc("/api/storage/sync", handler.HandleStorageSync)

// 2. RUTE DATA (Gunakan akhiran slash agar bisa baca ID di URL)
http.HandleFunc("/api/tx/", handler.HandleGetTx)         // Menangkap /api/tx/ID
http.HandleFunc("/api/block/", handler.HandleGetBlock)   // Menangkap /api/block/HEIGHT
http.HandleFunc("/get_block", handler.HandleGetBlock)    // Legacy support

// 3. RUTE STATE & HISTORY
http.HandleFunc("/api/info", handler.HandleInfo)
http.HandleFunc("/api/state", handler.HandleGetState)
http.HandleFunc("/api/history", handler.HandleAddressHistory)
http.HandleFunc("/api/search", handler.HandleAddressHistory)

// 4. BENTENG PERTAHANAN (CATCH-ALL)
// Kita ganti rute "/api/" yang lama dengan ini agar tidak "bertabrakan" dengan rute AI
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    // Jika path diawali /api/ tapi tidak terdaftar di atas, lempar ke Core (Port 8080)
    if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
        handler.HandleProxyMining(w, r)
        return
    }

    // Jika user mengakses root (/)
    fmt.Fprintf(w, "Welcome to BVM Nexus Gateway, General! 🫡\nStatus: Sentinel Active")
})


	fmt.Println("📢 Nexus Gateway aktif di http://localhost:9092")
	// Gunakan nil karena kita menggunakan http.HandleFunc (DefaultServeMux)
	if err := http.ListenAndServe(":9092", nil); err != nil {
	    fmt.Printf("❌ Gagal menghidupkan gateway: %v\n", err)
  }

}
