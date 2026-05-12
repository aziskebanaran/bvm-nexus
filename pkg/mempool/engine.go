package mempool

import (
    "sync"
    "strings"
    "github.com/aziskebanaran/bvm-core/x/bvm/types"
	"github.com/aziskebanaran/bvm-lib/utils"
	"github.com/aziskebanaran/bvm-lib/constants"
)

// Tambahkan Store ke struct untuk resolusi identitas
type NexusMempool struct {
    mu         sync.RWMutex
    queue      []types.Transaction
    maxSize    int
    // Pointer ke Store Nexus untuk cek id:@username
    store      interface { Get(string, interface{}) error } 
}

func NewNexusMempool(size int, s interface { Get(string, interface{}) error }) *NexusMempool {
    return &NexusMempool{
        queue:   make([]types.Transaction, 0),
        maxSize: size,
        store:   s,
    }
}

// Ganti bagian AddL2 Jenderal dengan ini:
func (m *NexusMempool) AddL2(tx types.Transaction) bool {
    m.mu.Lock()
    defer m.mu.Unlock()

    if len(m.queue) >= m.maxSize { return false }

    // 🛡️ Gunakan Doktrin bvm-lib untuk Resolusi Identitas
    // Proteksi prefix otomatis menggunakan BuildNexusKey
    if strings.HasPrefix(tx.To, "@") || strings.HasPrefix(tx.To, "0x") {
        var resolved string
        key := utils.BuildNexusKey(constants.PrefixIdentity, tx.To)
        if err := m.store.Get(key, &resolved); err == nil && resolved != "" {
            tx.To = resolved
        }
    }

    m.queue = append(m.queue, tx)
    return true
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
