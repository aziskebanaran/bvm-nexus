package p2p

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
)

// LoadIdentity memuat kunci privat Sultan untuk libp2p
func LoadIdentity() (crypto.PrivKey, error) {
	data, err := os.ReadFile("internal/nexus_identity.json")
	if err != nil {
		return nil, err
	}
	var id struct {
		SecretKey string `json:"secret_key"`
	}
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	
	// Ambil seed 32-byte dari hex secret_key
	seed, err := hex.DecodeString(id.SecretKey[:64])
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalEd25519PrivateKey(append(seed, make([]byte, 32)...))
}

func StartGlobalDHT(ctx context.Context, listenPort int) (host.Host, *dht.IpfsDHT, error) {
	privKey, err := LoadIdentity()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal memuat identitas: %v", err)
	}

	// Membuat Host dengan identitas Sultan
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
	)
	if err != nil {
		return nil, nil, err
	}

	// Mode Server agar bertindak sebagai penyedia data rute (Bootnode)
	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, nil, err
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, nil, err
	}

	fmt.Printf("🌐 [DHT] Global Radar Aktif!\n")
	return h, kademliaDHT, nil
}
