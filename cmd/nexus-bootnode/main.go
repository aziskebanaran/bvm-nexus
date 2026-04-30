package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	nexusP2P "github.com/aziskebanaran/bvm-nexus/pkg/p2p"
	 "github.com/ipfs/go-log/v2"
)

func main() {
	log.SetAllLoggers(log.LevelFatal)

	fmt.Println("🛰️  BVM-NEXUS BOOTNODE (MERCUSUAR) STARTING...")
	fmt.Println("----------------------------------------------")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Jalankan DHT dalam mode Server murni
	// Kita gunakan port 4001 (Standar P2P Global) agar mudah diingat
	port := 4001
	h, _, err := nexusP2P.StartGlobalDHT(ctx, port)
	if err != nil {
		fmt.Printf("❌ Gagal menyalakan Bootnode: %v\n", err)
		return
	}

	fmt.Println("✅ Mercusuar Aktif dan Mendengarkan...")
	fmt.Println("📢 Bagikan alamat di bawah ini ke node lain:")
	
	// Tampilkan semua alamat (Lokal & Publik jika ada)
	for _, addr := range h.Addrs() {
		fmt.Printf("🔗 %s/p2p/%s\n", addr, h.ID().String())
	}
	fmt.Println("----------------------------------------------")

	// Menjaga agar program tetap jalan sampai dihentikan (Ctrl+C)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("\n🛑 Mercusuar dihentikan. Sampai jumpa, Jenderal!")
}
