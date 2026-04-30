package mempool

import (
    "sync"
    "github.com/aziskebanaran/bvm-core/x/bvm/types"
)

type NexusMempool struct {
    mu         sync.RWMutex
    queue      []types.Transaction
    maxSize    int
}

func NewNexusMempool(size int) *NexusMempool {
    return &NexusMempool{
        queue:   make([]types.Transaction, 0),
        maxSize: size,
    }
}

// Add: Menampung transaksi dari Wallet/Miner
func (m *NexusMempool) Add(tx types.Transaction) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if len(m.queue) >= m.maxSize {
        return false // Mempool penuh
    }
    
    m.queue = append(m.queue, tx)
    return true
}

// Flush: Mengambil semua transaksi untuk dikirim ke Core
func (m *NexusMempool) Flush() []types.Transaction {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    txs := m.queue
    m.queue = make([]types.Transaction, 0) // Kosongkan antrean
    return txs
}
