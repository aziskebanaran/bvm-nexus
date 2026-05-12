package utxo

// UTXO adalah struktur tunggal untuk Koin dan Item Game di Nexus
type UTXO struct {
    // --- Identitas Finansial ---
    TxID      string `json:"txid"`      // ID Transaksi asal (Core L1 atau Nexus L2)
    Index     int    `json:"index"`     // Indeks kepingan (0, 1, dst)
    Address   string `json:"address"`   // Pemilik (bvmf, 0x, atau @username)
    Amount    uint64 `json:"amount"`    // Nilai koin (0 jika ini hanya Item Game)
    
    // --- Atribut Game (Flexibility) ---
    AssetID   string `json:"asset_id"`  // Contoh: ASSET-10540
    Metadata  string `json:"metadata"`  // Nama Item/Karakter
    Type      string `json:"type"`      // "COIN" atau "GAME_ITEM"
    
    // --- Status & Waktu ---
    Status    string `json:"status"`    // "UNSPENT" atau "SPENT"
    Timestamp int64  `json:"timestamp"` 
}

// Input adalah referensi untuk menghancurkan kepingan lama
type Input struct {
	PrevTxID string `json:"prev_txid"`
	Index    int    `json:"index"`
	Signature string `json:"signature"` // Bukti sah pemilik menghancurkan kepingan
}

// Output adalah kepingan baru yang akan dicetak
type Output struct {
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

// Transaction adalah proses penghancuran & pencetakan kepingan
type Transaction struct {
	ID        string   `json:"id"`
	Inputs    []Input  `json:"inputs"`
	Outputs   []Output `json:"outputs"`
	Fee       uint64   `json:"fee"`
	Timestamp int64    `json:"timestamp"`
}
