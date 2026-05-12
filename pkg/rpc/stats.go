package rpc

import (
    "sync"
    "time"
)

// MinerStat: Catatan kinerja per individu miner
type MinerStat struct {
    LastSeen   time.Time `json:"last_seen"`
    Hashrate   float64   `json:"hashrate"` // Hashes per second
    TotalFound uint64    `json:"total_found"`
}

// StatsManager: Gudang data statistik (In-Memory untuk kecepatan)
type StatsManager struct {
    mu     sync.RWMutex
    Miners map[string]*MinerStat
}

var GlobalStats = &StatsManager{
    Miners: make(map[string]*MinerStat),
}

// UpdateStat: Mencatat laporan masuk dari Miner
func (s *StatsManager) UpdateStat(addr string, hashes uint64, duration float64) {
    s.mu.Lock()
    defer s.mu.Unlock()

    hr := float64(hashes) / duration
    if _, ok := s.Miners[addr]; !ok {
        s.Miners[addr] = &MinerStat{}
    }
    
    s.Miners[addr].LastSeen = time.Now()
    s.Miners[addr].Hashrate = hr
}

// GetGlobalHashrate: Menghitung total kekuatan seluruh armada Jenderal
func (s *StatsManager) GetGlobalHashrate() float64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var total float64
    for _, m := range s.Miners {
        // Hanya hitung miner yang aktif dalam 1 menit terakhir
        if time.Since(m.LastSeen) < time.Minute {
            total += m.Hashrate
        }
    }
    return total
}
