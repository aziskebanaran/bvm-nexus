package vm

import (
        "fmt"
        "os" // 🚩 Tambahkan ini untuk ReadFile

        "github.com/aziskebanaran/bvm-lib/constants" // 🚩 Tambahkan ini
        "github.com/aziskebanaran/bvm-lib/sdk"
	"github.com/aziskebanaran/bvm-core/pkg/storage"
        "github.com/aziskebanaran/bvm-lib/utils"     // 🚩 Tambahkan ini
)


type NativeContract func(ctx sdk.Context, params []byte) ([]byte, error)

type WASMEngine struct {
	ContractPath    string
	DB             storage.BVMStore
	NativeRegistry  map[string]NativeContract // Daftar kontrak internal
}

func NewWASMEngine(path string, db storage.BVMStore) *WASMEngine {
	engine := &WASMEngine{
		ContractPath:   path,
		DB:             db,
		NativeRegistry: make(map[string]NativeContract),
	}
	engine.registerDefaults() // Daftarkan fungsi bawaan
	return engine
}

func (e *WASMEngine) registerDefaults() {
    e.NativeRegistry["PURCHASE_ASSET"] = func(ctx sdk.Context, params []byte) ([]byte, error) {
        // 1. Parsing Memo (Parameter)
        memo := string(params)
        assetID := "UNKNOWN"
        if len(memo) > 10 {
            assetID = memo[10:]
        }

        // 2. Membuat ID Unik untuk UTXO
        // 🚩 PERBAIKAN: Gunakan pengecekan panjang TxHash untuk mencegah crash
        safeTxID := "GENESIS"
        if len(ctx.TxHash) >= 16 {
            safeTxID = ctx.TxHash[:16]
        }
        utxoID := fmt.Sprintf("%s-0", safeTxID)

        // 3. Membangun Key dan Value dalam format []byte
        dbKey := []byte(utils.BuildSubKey(constants.PrefixUTXO, ctx.Sender, utxoID))
        dbVal := []byte(assetID)

        // 4. Eksekusi Penyimpanan ke Database Nexus
        err := e.DB.PutRaw(dbKey, dbVal)
        if err != nil {
            return nil, fmt.Errorf("gagal menyimpan UTXO: %v", err)
        }

        // 🚩 PERBAIKAN: Gunakan pengecekan panjang untuk Log agar terminal tetap stabil
        safeLogTx := "INTERNAL"
        if len(ctx.TxHash) >= 8 {
            safeLogTx = ctx.TxHash[:8]
        }

        fmt.Printf("🎰 [VM] Berhasil Minting %s untuk %s (TX: %s)\n", assetID, ctx.Sender, safeLogTx)
        return []byte("SUCCESS"), nil
    }
}

func (e *WASMEngine) ExecuteNative(method string, ctx sdk.Context, params []byte) ([]byte, error) {
	if contract, ok := e.NativeRegistry[method]; ok {
		return contract(ctx, params)
	}
	return nil, fmt.Errorf("kontrak %s tidak ditemukan", method)
}


func (e *WASMEngine) ExecuteContract(ctx sdk.Context, method string, params []byte) ([]byte, error) {
        // 1. Baca file WASM
        wasmBytes, err := os.ReadFile(e.ContractPath)
        if err != nil {
                return nil, fmt.Errorf("gagal membaca kontrak: %v", err)
        }

        // 2. Gunakan wasmBytes (Sementara kita cetak ukurannya agar Go tidak komplain)
        fmt.Printf("🚀 [VM-EXEC] Menjalankan %s (Size: %d bytes) untuk %s\n",
                method, len(wasmBytes), ctx.ContractAddr)

        // TODO: Kirim wasmBytes ke Wazero/Wasmer runtime

        return []byte("SUCCESS"), nil
}
