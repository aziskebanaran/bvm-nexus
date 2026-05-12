package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    "encoding/json"

    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/pkg/p2p"
    "github.com/aziskebanaran/bvm-core/x/wasm/keeper"

    nexusP2P "github.com/aziskebanaran/bvm-nexus/pkg/p2p"
    "github.com/aziskebanaran/bvm-nexus/pkg/utxo"
    "github.com/aziskebanaran/bvm-nexus/pkg/rpc"
    "github.com/aziskebanaran/bvm-nexus/pkg/state"
    "github.com/aziskebanaran/bvm-nexus/pkg/snapshot"
    "github.com/aziskebanaran/bvm-nexus/pkg/mempool" // 🚩 Pastikan ini di-import
    "github.com/ipfs/go-log/v2"
"github.com/aziskebanaran/bvm-nexus/pkg/vm"
)

func main() {
    log.SetAllLoggers(log.LevelFatal)
    fmt.Println("🌐 BVM-NEXUS: Modular Mode Activated")

    // 1. Database Nexus
    nexusStore, err := storage.NewLevelDBStore("./data_nexus/blockchain_db", 8)
    if err != nil {
        panic(err)
    }
    defer nexusStore.Close()

    // 🚩 1.5 INISIALISASI VM ENGINE (Personil Baru)
    // Kita buat engine di sini agar bisa dibagikan ke Sync dan RPC
    vmEngine := vm.NewWASMEngine("./build/node_manager.wasm", nexusStore)
    fmt.Println("🤖 [VM] Native & WASM Engine siap beroperasi.")

    // 2. Sentinel (Wasm Keeper)
    wk := keeper.NewKeeper(nexusStore)
    fmt.Println("🧠 [SENTINEL] Unit Keeper berhasil diaktifkan di Nexus.")

    // 3. P2P & Core Client
    go p2p.StartNode(9091)
    coreAddr := "http://localhost:8080"
    c := client.NewBVMClient(coreAddr)

    // 4. Background Sync (🚩 SEKARANG DENGAN ARGUMEN LENGKAP)
    // Kita kirim 'vmEngine' sebagai argumen ke-4
    go state.StartBatchSync(c, nexusStore, wk, vmEngine)

    // 5. Global DHT (Global Radar)
    ctx := context.Background()
    dhtHost, _, err := nexusP2P.StartGlobalDHT(ctx, 9093)
    if err != nil {
        fmt.Printf("⚠️ Gagal menyalakan DHT: %v\n", err)
    } else {
        defer dhtHost.Close()
    }

    // 6. 🚩 INISIALISASI MEMPOOL & RELAYER (Pondasi Ekonomi)
    nexusMempool := mempool.NewNexusMempool(1000, nexusStore) 
    go mempool.StartRelayer(nexusMempool, coreAddr)
    fmt.Println("🚀 [MEMPOOL] Antrean transaksi L2 & Relayer Aktif.")

    // 7. 🚩 AUTO-BOOTSTRAP & CLEANUP (Kesehatan Jaringan - Unit yang tadi sempat hilang)
    go nexusP2P.AutoBootstrap(":9092")
    go func() {
        fmt.Println("🧹 [SYSTEM] Petugas pembersihan peer aktif.")
        for {
            nexusP2P.CleanInactivePeers()
            time.Sleep(24 * time.Hour)
        }
    }()

	utxoStore := utxo.NewUTXOStore(nexusStore)

    // 8. API HANDLER SETUP (Integrasi Total)
    handler := &rpc.NexusHandler{
        Store:      nexusStore,
	UTXOStore: utxoStore,
        CoreURL:    coreAddr,
        CoreClient: c,
        Mempool:    nexusMempool, 
	// VM:      vmEngine,
    }

// --- REGISTRASI ROUTE (EDISI GABUNGAN TERKUAT) ---

// 1. RUTE SPESIFIK (Dahulukan rute yang butuh penanganan khusus)
http.HandleFunc("/api/discover-core", handler.MinerDiscoveryHandler)
http.HandleFunc("/api/login", handler.HandleProxyAuth)
http.HandleFunc("/api/nexus/status", handler.HandleNexusStatus)
http.HandleFunc("/api/nexus/update-peers", handler.HandleUpdatePeers)
http.HandleFunc("/peers", handler.HandleGetPeers)
http.HandleFunc("/p2p/register", handler.HandleRegisterPeer)
http.HandleFunc("/api/resolve", handler.HandleResolve)
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
http.HandleFunc("/api/holders", handler.HandleGetHolders)
// 🚩 PERUBAHAN DI SINI: Ganti dari HandleAddressHistory ke HandleSearch
http.HandleFunc("/api/search", handler.HandleSearch)
// 🚩 TAMBAHKAN JUGA RUTE UTXO AGAR WALLET BISA CEK KEPINGAN ASET
http.HandleFunc("/api/utxos", handler.HandleGetUTXOs)

// 🚩 TAMBAHKAN RUTE EKONOMI GAME DISINI:
http.HandleFunc("/api/market/list", handler.HandleMarketList)
http.HandleFunc("/api/market/buy", handler.HandleMarketBuy)
http.HandleFunc("/api/utxo/transfer", handler.HandleTransferUTXO) 

http.HandleFunc("/api/inventory/", handler.HandleGetInventory)
http.HandleFunc("/api/render/", handler.HandleRenderAsset)
http.HandleFunc("/api/utxo/spend", handler.HandleAutoSpend)
http.HandleFunc("/api/repair", handler.HandleRepairInventory)

// --- TAMBAHKAN DI SEKITAR RUTE DEX LAINNYA ---
http.HandleFunc("/api/dex/swap", handler.HandleSwap)
http.HandleFunc("/api/dex/add-liquidity", handler.HandleAddLiquidity)    // 🚩 Tambahkan ini
http.HandleFunc("/api/dex/remove-liquidity", handler.HandleRemoveLiquidity) // 🚩 Tambahkan ini

http.HandleFunc("/api/dex/vault-status", func(w http.ResponseWriter, r *http.Request) {
    // Alamat Vault didefinisikan di pkg/rpc/dex_handler.go
    vaultInfo := handler.BuildWalletMetadata("bvmf_dex_reserve_vault_system")
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(vaultInfo)
})

// 🚩 TAMBAHKAN RUTE INTELIJEN MINING DISINI:
http.HandleFunc("/api/stats/report-hash", handler.PostReportHash)
http.HandleFunc("/api/mining-stats", handler.GetMiningStats)
http.HandleFunc("/api/bridge/mint", handler.HandleBridgeMint)

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
