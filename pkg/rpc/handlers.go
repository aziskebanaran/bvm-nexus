package rpc

import (
    "encoding/json"
    "io"
    "net/http"
    "time"
	"os"
    "fmt" // Tambahkan ini

    "github.com/aziskebanaran/bvm-core/pkg/wallet"
    "github.com/aziskebanaran/bvm-nexus/pkg/p2p" // Tambahkan ini (panggil p2p nexus)
    "github.com/aziskebanaran/bvm-core/pkg/storage"
    "github.com/aziskebanaran/bvm-core/x/bvm/types"
    "github.com/libp2p/go-libp2p/core/crypto" // Tambahkan ini
)

type NexusHandler struct {
    Store storage.BVMStore
    CoreURL  string
}

func (h *NexusHandler) HandleProxyMining(w http.ResponseWriter, r *http.Request) {
    // 🚩 Gunakan h.CoreURL agar fleksibel (8080)
    coreURL := h.CoreURL + r.URL.Path
    if r.URL.RawQuery != "" {
        coreURL += "?" + r.URL.RawQuery
    }

    req, err := http.NewRequest(r.Method, coreURL, r.Body)
    if err != nil {
        http.Error(w, "❌ Gagal membuat request proxy", 500)
        return
    }

    // Copy Header (Penting untuk Token Bearer/Signature)
    for name, values := range r.Header {
        for _, value := range values {
            req.Header.Add(name, value)
        }
    }

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(502)
        json.NewEncoder(w).Encode(map[string]string{"status": "ERROR", "error": "Core BVM Offline"})
        return
    }
    defer resp.Body.Close()

    // Meneruskan apapun yang dijawab Core (Header & Body)
    for k, vv := range resp.Header {
        for _, v := range vv {
            w.Header().Add(k, v)
        }
    }
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}


func (h *NexusHandler) HandleGetBlock(w http.ResponseWriter, r *http.Request) {
    heightStr := r.URL.Query().Get("height")
    if heightStr == "" {
        http.Error(w, "Missing 'height' parameter", 400)
        return
    }

    var block types.Block
    // 🚩 Koreksi: Pastikan key sesuai dengan storage.go di Core
    err := h.Store.Get("b:"+heightStr, &block)
    if err != nil || block.Hash == "" {
        http.Error(w, "🔍 Blok tidak ditemukan di Database Nexus", 404)
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


func (h *NexusHandler) HandleCreateWallet(w http.ResponseWriter, r *http.Request) {
    // 1. Panggil fungsi asli dari Core Go Jenderal
    // Ini akan menghasilkan alamat 'bvmf...' dengan hash 10 bytes yang sah
    newW, mnemonic, err := wallet.CreateNewWallet()
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Gagal mencetak wallet resmi"})
        return
    }

    // 2. Kirim data resmi ke SDK (TS)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "address":     newW.Address,
        "private_key": newW.PrivateKey,
        "public_key":  newW.PublicKey,
        "mnemonic":    mnemonic,
        "status":      "OFFICIAL_BVM_CORE",
    })
}

func (h *NexusHandler) HandleAddressHistory(w http.ResponseWriter, r *http.Request) {
    // 1. Ambil query dari Wallet (q atau address)
    query := r.URL.Query().Get("q")
    if query == "" {
        query = r.URL.Query().Get("address")
    }

    // 2. Tanya ke Core (Port 8080)
    // Gunakan rute /api/history yang ada di router.go Core Jenderal
    resp, err := http.Get(h.CoreURL + "/api/history?address=" + query)
    if err != nil {
        http.Error(w, "Core Offline", 502)
        return
    }
    defer resp.Body.Close()

    // 3. Decode data mentah dari Core (satuan uint64)
    var coreTxs []types.Transaction
    if err := json.NewDecoder(resp.Body).Decode(&coreTxs); err != nil {
        // Jika gagal decode array, mungkin Core kirim error
        io.Copy(w, resp.Body)
        return
    }

    // 4. MAPPING SULTAN: Ubah ke format TransactionMetadata (float64)
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

    var result []TransactionMetadata
    for _, tx := range coreTxs {
        result = append(result, TransactionMetadata{
            ID:        tx.ID,
            From:      tx.From,
            To:        tx.To,
            Amount:    float64(tx.Amount) / 100000000.0, // Konversi ke desimal
            Symbol:    tx.Symbol,
            Memo:      tx.Memo,
            Timestamp: tx.Timestamp,
            Type:      tx.Type,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
