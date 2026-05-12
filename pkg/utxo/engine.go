package utxo

import "fmt"

func VerifyTransaction(tx Transaction, availableUTXOs []UTXO) error {
	var totalInput uint64 = 0
	var totalOutput uint64 = 0

	// 1. Hitung total nilai kepingan yang dihancurkan (Input)
	for _, in := range tx.Inputs {
		found := false
		for _, u := range availableUTXOs {
			if u.TxID == in.PrevTxID && u.Index == in.Index {
				totalInput += u.Amount
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("kepingan input tidak ditemukan atau sudah terpakai")
		}
	}

	// 2. Hitung total nilai kepingan yang dicetak (Output)
	for _, out := range tx.Outputs {
		totalOutput += out.Amount
	}

	// 3. Hukum Kekekalan Koin: Input = Output + Fee
	if totalInput != (totalOutput + tx.Fee) {
		return fmt.Errorf("jumlah input dan output tidak seimbang (potensi inflasi ilegal)")
	}

	return nil
}

// SpendUTXO mencari kepingan yang cukup untuk dikirim
func (s *UTXOStore) SpendUTXO(fromAddr string, amount uint64) ([]Input, uint64, error) {
	allUTXOs, err := s.GetUTXOsByAddress(fromAddr)
	if err != nil || len(allUTXOs) == 0 {
		return nil, 0, fmt.Errorf("saldo tidak ditemukan")
	}

	var selectedInputs []Input
	var totalValue uint64 = 0

	for _, u := range allUTXOs {
		selectedInputs = append(selectedInputs, Input{
			PrevTxID: u.TxID,
			Index:    u.Index,
		})
		totalValue += u.Amount
		
		// Jika sudah cukup, berhenti mencari
		if totalValue >= amount {
			break
		}
	}

	if totalValue < amount {
		return nil, 0, fmt.Errorf("saldo tidak cukup (butuh %d, cuma ada %d)", amount, totalValue)
	}

	return selectedInputs, totalValue, nil
}

// ConsolidateUTXOs menggabungkan banyak kepingan kecil menjadi satu kepingan besar
func (s *UTXOStore) ConsolidateUTXOs(addr string) (*Transaction, error) {
    utxos, err := s.GetUTXOsByAddress(addr)
    if err != nil || len(utxos) < 2 {
        return nil, fmt.Errorf("kepingan terlalu sedikit untuk dikonsolidasi")
    }

    var total uint64
    var inputs []Input
    for _, u := range utxos {
        total += u.Amount
        inputs = append(inputs, Input{
            PrevTxID: u.TxID,
            Index:    u.Index,
        })
    }

    // Biaya konsolidasi (Fee kecil untuk kebersihan database)
    fee := uint64(1000) 
    if total <= fee {
        return nil, fmt.Errorf("saldo terlalu kecil untuk biaya konsolidasi")
    }

    return &Transaction{
        Inputs:  inputs,
        Outputs: []Output{{To: addr, Amount: total - fee}},
        Fee:     fee,
    }, nil
}
