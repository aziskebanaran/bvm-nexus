package utxo

import (
        "encoding/json"
        "fmt"
        "github.com/aziskebanaran/bvm-core/pkg/storage"
        "github.com/aziskebanaran/bvm-lib/constants" // 🚩 IMPORT DOKTRIN JENDERAL
)

type UTXOStore struct {
        db storage.BVMStore
}

func NewUTXOStore(db storage.BVMStore) *UTXOStore {
        return &UTXOStore{db: db}
}

// Simpan menggunakan PrefixUTXO dari bvm-lib ("utxo:")
func (s *UTXOStore) SaveUTXO(u UTXO) error {
        // 🚩 GUNAKAN constants.PrefixUTXO agar sinkron global
        key := fmt.Sprintf("%s%s:%s:%d", constants.PrefixUTXO, u.Address, u.TxID, u.Index)
        return s.db.Put(key, u)
}

func (s *UTXOStore) RemoveUTXO(addr, txid string, index int) error {
        key := fmt.Sprintf("%s%s:%s:%d", constants.PrefixUTXO, addr, txid, index)
        return s.db.Delete(key)
}

func (s *UTXOStore) GetUTXOsByAddress(addr string) ([]UTXO, error) {
    var utxos []UTXO
    // 🚩 SCAN menggunakan prefix standar Jenderal
    prefix := fmt.Sprintf("%s%s:", constants.PrefixUTXO, addr)

    results, err := s.db.PrefixScan(prefix)
    if err != nil {
        return nil, err
    }

    for _, data := range results {
        var u UTXO
        if err := json.Unmarshal(data, &u); err == nil {
            utxos = append(utxos, u)
        }
    }
    return utxos, nil
}
