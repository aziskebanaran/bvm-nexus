# BVM Nexus Makefile
# Digunakan untuk membangun Gateway dan Bootnode

BINARY_NEXUS = nexusd
BINARY_BOOT = nexus-bootnode
SOURCE_NEXUS = ./cmd/nexusd/main.go
SOURCE_BOOT = ./cmd/nexus-bootnode/main.go

.PHONY: all build build-nexus init build-boot clean help

all: build

# Membangun semua komponen
build: build-nexus build-boot

build-nexus:
	@echo "🔨 Membangun BVM Nexus Gateway..."
	go build -o $(BINARY_NEXUS) $(SOURCE_NEXUS)
	@echo "✅ Selesai: ./$(BINARY_NEXUS)"

init:
	@echo "🌐 Menyiapkan Identitas Baru BVM Nexus..."
	go run internal/generator_kunci.go

build-boot:
	@echo "🔨 Membangun BVM Nexus Bootnode (Mercusuar)..."
	go build -o $(BINARY_BOOT) $(SOURCE_BOOT)
	@echo "✅ Selesai: ./$(BINARY_BOOT)"

clean:
	@echo "🧹 Membersihkan binary dan log..."
	rm -f $(BINARY_NEXUS) $(BINARY_BOOT) nexus.log
	@echo "✨ Bersih!"

help:
	@echo "Pusat Kendali Nexus:"
	@echo "  make build       - Membangun semua binary"
	@echo "  make clean       - Menghapus binary dan log"
