# 🌐 BVM Nexus: Modular Networking Layer

BVM Nexus adalah gerbang (gateway) dan lapisan jaringan modular untuk ekosistem **BVM (BVM Virtual Machine)**. Proyek ini berfungsi sebagai jembatan komunikasi P2P, manajemen Mempool, dan sistem Sinkronisasi State menggunakan unit **SENTINEL**.

## 🏗️ Arsitektur
* **Nexus Gateway (`nexusd`)**: Berjalan di port `9092`. Mengelola RPC, sinkronisasi ke Core, dan validasi blok via Wasm Keeper.
* **Nexus Bootnode (`nexus-bootnode`)**: Berjalan di port `4001`. Berfungsi sebagai "Mercusuar" P2P agar antar node dapat saling menemukan melalui DHT.

## 🚀 Prasyarat Sistem
Karena proyek ini menggunakan referensi kode lokal, pastikan struktur direktori Anda sebagai berikut:
```text
/home/
  ├── bvm-core/
  ├── bvm-lib/
  └── bvm-nexus/  <-- (Anda berada di sini)
