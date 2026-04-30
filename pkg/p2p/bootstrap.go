package p2p

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"net"
	"time"

	"github.com/aziskebanaran/bvm-core/pkg/constants"
	"github.com/aziskebanaran/bvm-core/pkg/p2p"
	"github.com/aziskebanaran/bvm-core/x/bvm/types"
	"github.com/hashicorp/mdns"
	"github.com/libp2p/go-libp2p/core/peer"
         multiaddr "github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p/core/crypto"
)

type SecureHandshakeRequest struct {
    NodeID    string `json:"node_id"`
    Signature []byte `json:"signature"`
    Timestamp int64  `json:"timestamp"`
    P2PPort   int    `json:"p2p_port"`
    APIPort   int    `json:"api_port"`
    Version   string `json:"version"`
}

type NexusIdentity struct {
	PeerID string `json:"peer_id"`
}


func AutoBootstrap(nexusAddr string) {
	myID := getMyNodeID()

	// 1. Jalankan P2P Manual (Fallback dari peers.json)
	peers := loadPeersFromNexus()
	for _, p := range peers {
		go connectToPeer(myID, p.Address)
	}

	// 2. Jalankan Radar mDNS (Otomatis)
	go startMDNS(myID)
}

func startMDNS(myID string) {
    // 1. Ambil IP Lokal secara manual (Termux Friendly)
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        fmt.Printf("⚠️ [mDNS] Gagal akses interface: %v\n", err)
        return
    }

    var ip net.IP
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                ip = ipnet.IP
                break
            }
        }
    }

    if ip == nil {
        fmt.Println("⚠️ [mDNS] Tidak menemukan IP aktif (Cek WiFi).")
        return
    }

    // 2. Setup Iklan dengan IP yang ditemukan
    info := []string{"peer_id=" + myID}
    service, err := mdns.NewMDNSService(myID, "_bvm-nexus._tcp", "", "", 9092, []net.IP{ip}, info)
    if err != nil {
        fmt.Printf("⚠️ [mDNS] Gagal setup iklan: %v\n", err)
        return
    }

    server, _ := mdns.NewServer(&mdns.Config{Zone: service})
    defer server.Shutdown()

    fmt.Printf("📢 [mDNS] RADAR AKTIF di IP: %s (ID: %s)\n", ip.String(), myID)

    // 3. Loop Discovery (Mencari teman)
    for {
        fmt.Println("🔍 [mDNS] Memindai jaringan WiFi...")
        entriesCh := make(chan *mdns.ServiceEntry, 10)

        go func() {
            for entry := range entriesCh {
                if !strings.Contains(entry.Name, myID) {
                    peerAddr := fmt.Sprintf("%s:%d", entry.AddrV4.String(), entry.Port)
                    fmt.Printf("✨ [mDNS] Menemukan Peer: %s\n", peerAddr)
                    connectToPeer(myID, peerAddr)
                }
            }
        }()

        mdns.Lookup("_bvm-nexus._tcp", entriesCh)
        close(entriesCh)
        time.Sleep(30 * time.Second)
    }
}

func connectToPeer(myID string, targetAddr string) {
    // 1. Cek apakah ini Multiaddress (DHT)
    if strings.HasPrefix(targetAddr, "/") {
        ma, err := multiaddr.NewMultiaddr(targetAddr)
        if err != nil { return }
        
        peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
        if err != nil { return }

        fmt.Printf("✨ [DHT] Terhubung ke Mercusuar: %s\n", peerInfo.ID.String())
        savePeerToNexus(targetAddr)
        return // Selesai untuk DHT
    }

    // 2. Jika IP Biasa, coba Secure Handshake
    privKey, err := getMyPrivateKey()
    if err != nil {
        fmt.Printf("⚠️ [SECURITY] Kunci rahasia tak ditemukan, kirim salam biasa ke: %s\n", targetAddr)
        
        // Logika Fallback (Ganti 'performLegacyHandshake' yang error tadi)
        p2p.SendToPeer(targetAddr, "/api/peers", p2p.HandshakeRequest{
            NodeID:      myID,
            P2PPort:     9091,
            APIPort:     9092,
            Version:     constants.ProjectVersion,
            GenesisHash: "BVM_GENESIS_001",
        })
        return
    }

    // 3. Jalankan Secure Handshake
    timestamp := time.Now().Unix()
    msg := fmt.Sprintf("%s:%d", myID, timestamp)
    sig, _ := privKey.Sign([]byte(msg))

    p2p.SendToPeer(targetAddr, "/api/secure-handshake", SecureHandshakeRequest{
        NodeID:    myID,
        Signature: sig,
        Timestamp: timestamp,
        P2PPort:   9091,
        APIPort:   9092,
        Version:   constants.ProjectVersion,
    })
}


func getMyNodeID() string {
	const idFile = "internal/nexus_identity.json"
	data, err := os.ReadFile(idFile)
	if err != nil {
		return "NEXUS_GENERIC"
	}
	var identity NexusIdentity
	json.Unmarshal(data, &identity)
	if identity.PeerID == "" {
		return "NEXUS_UNKNOWN"
	}
	return "NEXUS-" + identity.PeerID[:8]
}

func loadPeersFromNexus() []types.Peer {
	const peerFile = "data_nexus/peers.json"
	data, err := os.ReadFile(peerFile)
	if err != nil {
		return []types.Peer{}
	}
	var peers []types.Peer
	json.Unmarshal(data, &peers)
	return peers
}

// savePeerToNexus menambahkan peer baru ke file peers.json jika belum ada
func savePeerToNexus(newAddr string) {
    const peerFile = "data_nexus/peers.json"

    // 1. Baca daftar yang sudah ada
    peers := loadPeersFromNexus()

    // 2. Cek apakah sudah ada (mencegah duplikasi)
    for _, p := range peers {
        if p.Address == newAddr {
            return // Sudah ada, tidak perlu simpan
        }
    }

    // 3. Tambahkan peer baru (menggunakan struct Peer dari bvm-core/types)
    peers = append(peers, types.Peer{
        Address:  newAddr,
        LastSeen: time.Now().Unix(),
    })

    // 4. Tulis kembali ke file
    data, _ := json.MarshalIndent(peers, "", "  ")
    err := os.WriteFile(peerFile, data, 0644)
    if err == nil {
        fmt.Printf("📝 [SYNC] Peer baru tersimpan: %s\n", newAddr)
    }
}

func CleanInactivePeers() {
	const peerFile = "data_nexus/peers.json"
	const masaBerlaku = 3 * 24 * 60 * 60 // 3 Hari dalam detik

	peers := loadPeersFromNexus()
	now := time.Now().Unix()
	var activePeers []types.Peer

	for _, p := range peers {
		// Jika terakhir terlihat kurang dari 3 hari, simpan
		if now-p.LastSeen < masaBerlaku {
			activePeers = append(activePeers, p)
		} else {
			fmt.Printf("🗑️ [CLEANUP] Menghapus peer tidak aktif: %s\n", p.Address)
		}
	}

	// Tulis ulang file jika ada yang dihapus
	if len(activePeers) != len(peers) {
		data, _ := json.MarshalIndent(activePeers, "", "  ")
		os.WriteFile(peerFile, data, 0644)
	}
}

func getMyPrivateKey() (crypto.PrivKey, error) {
    // Asumsi: Sultan menyimpan kunci di nexus_identity.json dalam format Base64
    // Jika Sultan menggunakan format libp2p identity, kita gunakan unmarshal
    const idFile = "internal/nexus_identity_key.dat" // Sesuaikan lokasi file kunci Sultan
    data, err := os.ReadFile(idFile)
    if err != nil {
        return nil, err
    }
    return crypto.UnmarshalPrivateKey(data)
}
