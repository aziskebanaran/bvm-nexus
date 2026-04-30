package rpc

import (
	"encoding/json"
	"net/http"
)

// Response untuk Miner yang sedang mencari jalan
type DiscoveryResponse struct {
	CoreAddress string `json:"core_address"`
	Latency     string `json:"latency"`
	Status      string `json:"status"`
}

// MinerDiscoveryHandler: Memberitahu Miner di mana Markas Core berada
func (h *NexusHandler) MinerDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	// 🎯 TAKTIK: Mengambil CoreURL langsung dari konfigurasi Handler Jenderal
	bestCore := h.CoreURL 
	
	if bestCore == "" {
		bestCore = "http://localhost:8080" // Fallback ke standar
	}

	response := DiscoveryResponse{
		CoreAddress: bestCore,
		Latency:     "0.1ms", // Di masa depan, ini bisa dinamis berdasarkan DHT
		Status:      "OPERATIONAL",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-BVM-Navigation", "Sultan-Global-GPS")
	json.NewEncoder(w).Encode(response)
}
