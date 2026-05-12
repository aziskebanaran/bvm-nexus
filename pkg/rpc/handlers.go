package rpc

import (
	"sort"
    "encoding/json"
    "io"
    "net/http"
    "time"
	"os"
    "fmt" // Tambahkan ini
	"strings"

    "github.com/aziskebanaran/bvm-lib/constants"
    "github.com/aziskebanaran/bvm-lib/utils"
    "github.com/aziskebanaran/bvm-core/pkg/client"
    "github.com/aziskebanaran/bvm-nexus/pkg/p2p" // Tambahkan ini (panggil p2p nexus)
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/x/bvm/types" 
    "github.com/libp2p/go-libp2p/core/crypto" // Tambahkan ini
    "github.com/aziskebanaran/bvm-nexus/pkg/rpc/user"
    "github.com/aziskebanaran/bvm-nexus/pkg/mempool"
    "github.com/aziskebanaran/bvm-nexus/pkg/utxo"
)

type NexusHandler struct {
    Store      storage.BVMStore
    UTXOStore *utxo.UTXOStore
    CoreURL    string
    CoreClient *client.BVMClient
    Mempool    *mempool.NexusMempool

}

func (h *NexusHandler) HandleProxyMining(w http.ResponseWriter, r *http.Request) {
    // 1. Tentukan Target (Core 8080)
    coreURL := h.CoreURL + r.URL.Path
    if r.URL.RawQuery != "" {
        coreURL += "?" + r.URL.RawQuery
    }

    // 2. Gunakan NewRequest dengan r.Body
    req, err := http.NewRequest(r.Method, coreURL, r.Body)
    if err != nil {
        http.Error(w, "❌ Gagal membuat request proxy", 500)
        return
    }

    // 3. SALIN HEADER (Krusial untuk Autentikasi)
    for name, values := range r.Header {
        for _, value := range values {
            req.Header.Add(name, value)
        }
    }

    // 4. Client dengan Timeout sedikit lebih longgar untuk mobile
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("⚠️ [NEXUS] Core Offline/Busy: %v\n", err)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(502)
        json.NewEncoder(w).Encode(map[string]string{"status": "ERROR", "error": "Core BVM Offline"})
        return
    }
    defer resp.Body.Close()

    // 5. SALIN BALIK HEADER DARI CORE
    for k, vv := range resp.Header {
        for _, v := range vv {
            w.Header().Add(k, v)
        }
    }

    // 6. KIRIM STATUS & BODY (Ini yang menghentikan EOF)
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

func (h *NexusHandler) HandleGetTx(w http.ResponseWriter, r *http.Request) {
    // Taktik: Ambil ID dari Query (?id=) atau dari Path (/api/tx/ID)
    txid := r.URL.Query().Get("id")
    if txid == "" {
        txid = strings.TrimPrefix(r.URL.Path, "/api/tx/")
    }

    if txid == "" || txid == "/api/tx" {
        http.Error(w, `{"error": "TXID required"}`, 400)
        return
    }

    displayID := txid
    if len(txid) >= 10 {
        displayID = txid[:10]
    }
    fmt.Printf("🕵️ [NEXUS] Mencari detail transaksi: %s\n", displayID)

    // 1. Cek Database Lokal Nexus (Hasil Crawling)
    var tx types.Transaction
    // Gunakan prefix "tx:" sesuai yang kita buat di sync.go tadi
    err := h.Store.Get(utils.BuildCoreKey(constants.DBTxPrefix, txid), &tx)

    if err == nil && tx.ID != "" {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Source", "NEXUS-CRAWLER-LOCAL")
        json.NewEncoder(w).Encode(tx)
        return
    }

    // 2. Fallback: Jika tidak ada di lokal, tanya ke Core (Proxy)
    targetURL := fmt.Sprintf("%s/api/tx/%s", h.CoreURL, txid)
    resp, err := http.Get(targetURL)
    if err != nil {
        http.Error(w, `{"error": "Core Offline"}`, 502)
        return
    }
    defer resp.Body.Close()

    w.Header().Set("Content-Type", "application/json")
    io.Copy(w, resp.Body)
}

func (h *NexusHandler) HandleGetBlock(w http.ResponseWriter, r *http.Request) {
    // Taktik: Ambil Height dari Query (?height=) atau Path (/api/block/8149)
    heightStr := r.URL.Query().Get("height")
    if heightStr == "" {
        heightStr = strings.TrimPrefix(r.URL.Path, "/api/block/")
    }

    if heightStr == "" || heightStr == "/api/block" {
        http.Error(w, "Missing block height", 400)
        return
    }

    fmt.Printf("🧱 [NEXUS] Mengambil data blok: #%s\n", heightStr)

    var block types.Block
    // Prefix "b:" adalah standar storage untuk blok
    err := h.Store.Get(utils.BuildCoreKey(constants.DBBlockPrefix, heightStr), &block)

    if err != nil || block.Hash == "" {
        // Fallback: Jika tidak ada di Nexus, lempar ke Core
        targetURL := fmt.Sprintf("%s/api/block/%s", h.CoreURL, heightStr)
        resp, err := http.Get(targetURL)
        if err != nil {
            http.Error(w, "Core Offline", 502)
            return
        }
        defer resp.Body.Close()
        w.Header().Set("Content-Type", "application/json")
        io.Copy(w, resp.Body)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(block)
}


func (h *NexusHandler) HandleInfo(w http.ResponseWriter, r *http.Request) {
    // 1. Ambil data mentah dari Core Status
    targetURL := h.CoreURL + "/api/status"
    resp, err := http.Get(targetURL)
    if err != nil {
        w.WriteHeader(502)
        json.NewEncoder(w).Encode(map[string]string{"error": "Core BVM Offline"})
        return
    }
    defer resp.Body.Close()

    // 2. Decode ke NodeStatus (Tipe asli dari Core)
    var status types.NodeStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        http.Error(w, "Gagal decode status core", 500)
        return
    }

    // 3. 🚩 RAKIT ULANG menggunakan 'types.NetworkResponse' (Tipe resmi dari Core)
    // Ini menjamin Wallet Sultan tidak akan pernah kena error 'unmarshal number' lagi
    response := types.NetworkResponse{
        Height:      status.Height,
        LatestHash:  status.LatestHash,
        Difficulty:  int(status.Difficulty),
        Reward:      status.Reward,
        DynamicFee:  1000,           // Standar fee
        MempoolSize: int(status.InFlight),
        NetworkName: "BVM Atomic Mainnet",
        // 🛡️ BAGIAN TERPENTING: Memasukkan objek Params yang dicari Wallet
        Params: types.DefaultParams(), 
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Nexus-Bridge", "Active")
    json.NewEncoder(w).Encode(response)
}

// HandleSecureHandshake memeriksa sertifikat digital dari node lain
func (h *NexusHandler) HandleSecureHandshake(w http.ResponseWriter, r *http.Request) {
    var req struct {
        NodeID    string `json:"node_id"`
        Signature []byte `json:"signature"`
        Timestamp int64  `json:"timestamp"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Bad Request", 400)
        return
    }

    // 1. Ekstrak Public Key dari NodeID (libp2p standard)
    // NodeID sebenarnya adalah hash dari Public Key
    pubKey, err := extractPubKeyFromNodeID(req.NodeID)
    if err != nil {
        http.Error(w, "🚫 Kunci Publik Tidak Valid", 401)
        return
    }

    // 2. Jalankan Verifikasi dari security.go (panggil package p2p)
    // Pastikan VerifyIdentity di security.go sudah bersifat Public (Huruf Kapital)
    if !p2p.VerifyIdentity(req.NodeID, req.Signature, req.Timestamp, pubKey) {
        http.Error(w, "🚫 Akses Ditolak: Identitas Tidak Sah", http.StatusUnauthorized)
        return
    }

    // 3. Jika lolos, simpan sebagai teman terpercaya
    fmt.Printf("🛡️ [SECURITY] Node %s lolos verifikasi!\n", req.NodeID)
    
    // Kirim balasan sukses
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "verified", "message": "Welcome to BVM Network"})
}


// HandleWalletSend meneruskan transaksi dari Wallet ke Core via Proxy
func (h *NexusHandler) HandleWalletSend(w http.ResponseWriter, r *http.Request) {
    fmt.Println("🚀 [NEXUS] Meneruskan transaksi transfer ke Core...")
    h.HandleProxyMining(w, r)
}


// Pastikan fungsi ini ada jika dipanggil di HandleSecureHandshake
func extractPubKeyFromNodeID(idStr string) (crypto.PubKey, error) {
    return nil, fmt.Errorf("fungsi belum diimplementasi sepenuhnya")
}


func (h *NexusHandler) HandleStorageSync(w http.ResponseWriter, r *http.Request) {
    // 1. Ambil Header Identitas (Ganti JWT dengan Signature)
    addr := r.Header.Get("X-BVM-Address")
    sig := r.Header.Get("X-BVM-Signature")
    msg := r.Header.Get("X-BVM-Message")
    appID := r.Header.Get("X-BVM-App-ID")

    if addr == "" || sig == "" {
        http.Error(w, `{"error": "Signature Required"}`, 401)
        return
    }

    // 2. Teruskan ke Core Utama Sultan (Port 8080)
    proxyReq, _ := http.NewRequest(r.Method, h.CoreURL+"/api/storage/put", r.Body)

    // Copy Header agar Core bisa memverifikasi tanda tangan user
    proxyReq.Header.Set("X-BVM-Address", addr)
    proxyReq.Header.Set("X-BVM-Signature", sig)
    proxyReq.Header.Set("X-BVM-Message", msg)
    proxyReq.Header.Set("X-BVM-App-ID", appID)
    proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))

    client := &http.Client{}
    resp, err := client.Do(proxyReq)
    if err != nil {
        http.Error(w, `{"error": "Core Offline"}`, 503)
        return
    }
    defer resp.Body.Close()

    // Kembalikan hasil dari Core ke Wallet
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

// ProxyToCore: Fungsi sapu jagat untuk meneruskan request apa saja ke Core
func (h *NexusHandler) ProxyToCore(w http.ResponseWriter, r *http.Request) {
    h.HandleProxyMining(w, r)
}

// HandleProxyAuth: Jalur khusus untuk Login & Autentikasi
func (h *NexusHandler) HandleProxyAuth(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("🔐 [NEXUS] Meneruskan Permintaan Auth ke Core: %s\n", h.CoreURL)

    // Kita gunakan mesin proxy yang sama dengan Mining agar praktis
    h.HandleProxyMining(w, r)
}

func (h *NexusHandler) HandleGetPeers(w http.ResponseWriter, r *http.Request) {
    // Membaca database peer Nexus (data_nexus/peers.json)
    data, err := os.ReadFile("data_nexus/peers.json")
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte("[]")) // Kembalikan array kosong jika file belum ada
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}

func (h *NexusHandler) HandleRegisterPeer(w http.ResponseWriter, r *http.Request) {
    var req struct {
        IP string `json:"ip"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid Payload", 400)
        return
    }

    // Logika: Tambahkan ke sistem P2P Nexus Jenderal
    fmt.Printf("🌐 [NEXUS] SDK Mendaftarkan Peer: %s\n", req.IP)

    w.WriteHeader(200)
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (h *NexusHandler) HandleNexusStatus(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    var latestHeight uint64

    // Taktik: Gunakan &latestHeight sebagai argumen kedua 
    // agar Store langsung mengisi nilainya ke variabel tersebut.
    err := h.Store.Get("latest_height", &latestHeight)

    if err != nil {
        // Jika gagal (key tidak ditemukan), biarkan 0 atau gunakan angka terakhir
        latestHeight = 0 
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":              "SENTINEL_OK",
        "latest_block_height": latestHeight,
    })
}

// HandleUpdatePeers menerima daftar peer dari SDK TS
func (h *NexusHandler) HandleUpdatePeers(w http.ResponseWriter, r *http.Request) {
    fmt.Println("📩 [NEXUS] Menerima intelijen peer dari SDK TS...")
    // Logika simpan peer ke database atau p2p list Jenderal di sini
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, "Intel received, General!")
}

func (h *NexusHandler) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
    // Taktik: Gunakan Proxy ke Core Port 8080 agar data tetap sinkron
    fmt.Println("💰 [NEXUS] Mengambil saldo dari Core...")
    h.HandleProxyMining(w, r) 
}


func (h *NexusHandler) HandleAddressHistory(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    if query == "" {
        query = r.URL.Query().Get("address")
    }

    finalAddress := query
    var resolved string
    // Resolusi mapping (0x atau @ ke bvmf)
    if err := h.Store.Get("id:"+query, &resolved); err == nil && resolved != "" {
        finalAddress = resolved
    }

    displayID := finalAddress
    if len(finalAddress) > 8 { displayID = finalAddress[:8] }
    fmt.Printf("🕵️ [NEXUS-INTEL] Memindai riwayat untuk: %s...\n", displayID)

    var history []types.Transaction

    // 1. Coba ambil dari lokal Nexus
    h.Store.Get(utils.BuildNexusKey(constants.PrefixHistory, finalAddress), &history)

    // 2. 🚩 FORCE SYNC: Jika lokal masih kosong, tarik dari Core
    if len(history) == 0 {
        fmt.Printf("🔄 [NEXUS] Memaksa Sinkronisasi Core untuk: %s\n", displayID)

        targetURL := fmt.Sprintf("%s/api/history?address=%s", h.CoreURL, finalAddress)
        resp, errR := http.Get(targetURL)
        if errR == nil {
            defer resp.Body.Close()

            var remoteHistory []types.Transaction
            if errD := json.NewDecoder(resp.Body).Decode(&remoteHistory); errD == nil && len(remoteHistory) > 0 {
                // 💾 SIMPAN KE ALAMAT ASLI (finalAddress)
                h.Store.Put("h:"+finalAddress, remoteHistory)
                history = remoteHistory
                fmt.Printf("✅ [NEXUS] %d transaksi Sultan diarsipkan!\n", len(history))
            }
        }
    }

    // Tambahan: Jika query tidak sama dengan finalAddress (misal query pakai 0x), 
    // simpan juga mapping history-nya agar pencarian lewat 0x juga cepat.
    if query != finalAddress && len(history) > 0 {
        h.Store.Put("h:"+query, history)
    }

    // --- 🚩 STAGE 3: MAPPING KE DASHBOARD (Tetap) ---
    type TransactionMetadata struct {
        ID        string  `json:"id"`
        From      string  `json:"from"`
        To        string  `json:"to"`
        Amount    float64 `json:"amount"`
        Symbol    string  `json:"symbol"`
        Memo      string  `json:"memo"`
        Timestamp int64   `json:"timestamp"`
        Type      string  `json:"type"`
    }

    var result []TransactionMetadata = make([]TransactionMetadata, 0) // Pastikan [] bukan null
    for _, tx := range history {
        result = append(result, TransactionMetadata{
            ID:        tx.ID,
            From:      tx.From,
            To:        tx.To,
            Amount:    float64(tx.Amount) / 100000000.0,
            Symbol:    tx.Symbol,
            Memo:      tx.Memo,
            Timestamp: tx.Timestamp,
            Type:      tx.Type,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}


func (h *NexusHandler) HandleGetState(w http.ResponseWriter, r *http.Request) {
    address := r.URL.Query().Get("address")

    if address == "" {
        http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
        return
    }

    type StateCache struct { Balance uint64; Nonce uint64 }
    var cache StateCache

    // 1. Ambil dari lokal
    err := h.Store.Get(utils.BuildNexusKey(constants.PrefixState, address), &cache)

    // Siapkan variabel temp dengan nilai dari cache (jika ada)
    balance := cache.Balance
    nonce := cache.Nonce

    // 🚩 2. SYARAT AGRESIF: Tarik dari Core jika saldo lokal masih 0
    if err != nil || balance == 0 || nonce == 0 {

        displayAddr := address
        if len(address) >= 8 {
            displayAddr = address[:8]
        }

        fmt.Printf("🔄 [NEXUS] Sinkronisasi saldo asli %s (Lokal: %d)...\n", displayAddr, balance)

        targetURL := fmt.Sprintf("%s/api/balance?address=%s", h.CoreURL, address)

        resp, errR := http.Get(targetURL)
        if errR == nil {
            defer resp.Body.Close()
            var rawData map[string]interface{}
            if errD := json.NewDecoder(resp.Body).Decode(&rawData); errD == nil {
                // Ambil balance_atomic
                if val, ok := rawData["balance_atomic"]; ok {
                    balance = convertToUint64(val)
                }
                // Ambil nonce
                if nVal, ok := rawData["nonce"]; ok {
                    nonce = convertToUint64(nVal)
                }

                // 🎯 SIMPAN HANYA JIKA HASIL DARI CORE > 0
                if balance > 0 || nonce > 0 {
                    h.Store.Put("s:"+address, StateCache{Balance: balance, Nonce: nonce})
                    fmt.Printf("✅ [NEXUS] Intelijen Berhasil! Bal: %d | Nonce: %d\n", balance, nonce)
                    // Update cache untuk respon JSON
                    cache.Balance = balance
                    cache.Nonce = nonce
                }
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "address": address,
        "balance": cache.Balance,
        "nonce":   cache.Nonce,
        "source":  "NEXUS_SYNCED_DATA",
    })
}

func (h *NexusHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error": "Query is required"}`, 400)
		return
	}

	// 1. Logika Resolusi Identitas (Mapping Pintar)
	// Kita tentukan alamat dasar (bvmf) dari input @username, 0x, atau alamat asli
	finalAddr := query
	var resolved string

	// Cek database identitas Nexus
	if err := h.Store.Get("id:"+query, &resolved); err == nil && resolved != "" {
		finalAddr = resolved
	}

	// 🚩 2. PANGGIL MESIN METADATA
	// Ini akan otomatis mengambil Saldo (s:), Nonce (n:), History (h:), dan UTXO
	metadata := h.BuildWalletMetadata(finalAddr)

	// 3. RAKIT RESPON TERPADU (Sinkron dengan Wallet CLI)
	// Pastikan semua key string sesuai dengan yang dipanggil bvm-wallet search
	report := map[string]interface{}{
		"query":           query,
		"resolved_addr":   metadata.Address,       // Mencegah <nil> di Alamat Asli
		"eth_addr":        metadata.MappedAddress, // Alamat 0x pasangannya
		"balance_atomic":  metadata.BalanceAtomic,
		"balance_bvm":     float64(metadata.BalanceAtomic) / 100000000.0,
		"nonce":           metadata.Nonce,
		"utxo_count":      metadata.UTXOCount,
		"tx_count":        metadata.ActivityCount,
		"status":          "ACTIVE",
	}

	// Sertakan aktivitas terakhir jika ada
	if metadata.LastSeen > 0 {
		report["last_activity"] = metadata.LastSeen
		report["last_memo"]     = metadata.LastMemo
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Sentinel-Source", "NEXUS-INTEL-L2")
	json.NewEncoder(w).Encode(report)
}


// HandleBalance menjawab permintaan Saldo spesifik
func (h *NexusHandler) HandleBalance(w http.ResponseWriter, r *http.Request) {
    fmt.Println("💰 [NEXUS] Meminta Saldo dari Core...")
    h.HandleProxyMining(w, r)
}

// Fungsi pembantu untuk konversi segala jenis tipe data JSON ke uint64
func convertToUint64(v interface{}) uint64 {
    switch t := v.(type) {
    case float64:
        return uint64(t)
    case string:
        // Jika saldo dikirim sebagai string "1000"
        var res uint64
        fmt.Sscanf(t, "%d", &res)
        return res
    case uint64:
        return t
    }
    return 0
}


func (h *NexusHandler) HandleGetHolders(w http.ResponseWriter, r *http.Request) {
    fmt.Println("🏆 [NEXUS] Menyusun Daftar Elit BVM (Rich List)...")

    // 1. Ambil data mentah dari Core
    resp, err := http.Get(h.CoreURL + "/api/holders")
    if err != nil {
        http.Error(w, "Core Offline", 502)
        return
    }
    defer resp.Body.Close()

    var rawHolders map[string]map[string]int64
    json.NewDecoder(resp.Body).Decode(&rawHolders)

    // 2. Transformasi ke Array agar bisa di-Sort
    type HolderInfo struct {
        Address  string  `json:"address"`
        Username string  `json:"username"`
        Balance  float64 `json:"balance"`
    }
    var eliteList []HolderInfo

    for addr, assets := range rawHolders {
        balanceUnit := assets["BVM"]

        // Lewati yang saldonya 0 atau alamat sistem internal jika ingin bersih
        if balanceUnit <= 0 || len(addr) > 60 { continue }

        // 🔍 Cek apakah alamat ini punya Username di database Nexus
        var username string
        if errU := h.Store.Get("name:"+addr, &username); errU != nil {
            username = "Anonymous Holder"
        }

        eliteList = append(eliteList, HolderInfo{
            Address:  addr,
            Username: username,
            Balance:  float64(balanceUnit) / 100000000.0, // Konversi ke BVM
        })
    }

    // 3. SORTING: Terkaya di atas!
    sort.Slice(eliteList, func(i, j int) bool {
        return eliteList[i].Balance > eliteList[j].Balance
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "network": "BVM-Mainnet",
        "total_holders": len(eliteList),
        "holders": eliteList,
    })
}


// Implementasi Helper Tinggi Blok
func (h *NexusHandler) GetLocalHeight() uint64 {
    var height uint64
    h.Store.Get("latest_height", &height)
    return height
}

// Perbaikan Distribusi Reward Miner
func (h *NexusHandler) CalculateMinerBonus(systemBalance uint64) uint64 {
    // Gunakan GetNetworkInfo karena GetParams tidak ada
    info, err := h.CoreClient.GetNetworkInfo()
    if err != nil { return 0 }
    
    params := info.Params
    currentHeight := h.GetLocalHeight()
    
    blocksLeft := int64(params.HalvingInterval) - (int64(currentHeight) % int64(params.HalvingInterval))
    if blocksLeft <= 0 { return 0 }
    
    return systemBalance / uint64(blocksLeft)
}



func (h *NexusHandler) HandleRenderAsset(w http.ResponseWriter, r *http.Request) {
    // Ambil ID setelah /api/render/
    parts := strings.Split(r.URL.Path, "/")
    id := ""
    if len(parts) > 3 {
        id = parts[3]
    }

    // Panggil unit renderer
    user.ExecuteRender(w, id)
}


// PostReportHash: Miner melaporkan hasil patrolinya (Versi Standard http)
func (h *NexusHandler) PostReportHash(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Metode dilarang, Jenderal!", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        MinerAddr string  `json:"miner_addr"`
        Hashes    uint64  `json:"hashes"`
        Duration  float64 `json:"duration"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Laporan cacat", http.StatusBadRequest)
        return
    }

    // Masukkan ke Stats Manager (GlobalStats harus bisa diakses di sini)
    GlobalStats.UpdateStat(req.MinerAddr, req.Hashes, req.Duration)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "Laporan diterima, Jenderal!"})
}

// GetMiningStats: Dashboard statistik
func (h *NexusHandler) GetMiningStats(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "global_hashrate": GlobalStats.GetGlobalHashrate(),
        "active_miners":   len(GlobalStats.Miners),
        "timestamp":       time.Now().Unix(),
    })
}
