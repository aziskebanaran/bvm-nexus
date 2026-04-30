#!/bin/bash

# Target Operasi: BVM Nexus Infrastructure
cd ~/bvm-nexus

echo "🌐 Memulai sinkronisasi BVM Nexus ke Awan..."

# 1. Menambahkan semua perubahan
git add .

# 2. Ambil Versi Terakhir & Hitung Versi Baru
# Mengambil tag terakhir (format vX.X.X), memecah angkanya, dan menambah 1
latest_tag=$(git tag --list 'v*' | sort -V | tail -n 1)
if [ -z "$latest_tag" ]; then
    next_tag="v1.0.0"
else
    # Mengambil angka terakhir dari format v1.1.X dan menambahkannya
    base_version=$(echo $latest_tag | cut -d. -f1,2)
    patch_version=$(echo $latest_tag | cut -d. -f3)
    next_tag="$base_version.$((patch_version + 1))"
fi

echo "🛰️  Versi Nexus terakhir : $latest_tag"
echo "🆕 Menyiapkan versi baru  : $next_tag"

# 3. Pesan Komando
echo "📝 Apa pesan untuk versi $next_tag ini, Jenderal?"
read message

# Jika pesan kosong, beri pesan default
if [ -z "$message" ]; then
    message="Update Nexus Infrastructure"
fi

# 4. Eksekusi Git (Commit, Tag, Push)
git commit -m "$next_tag: $message"
git tag -a "$next_tag" -m "$message"

echo "📡 Mengirim transmisi Nexus ke GitHub..."
git push origin main
git push origin "$next_tag"

echo "---------------------------------------"
echo "✅ MISI NEXUS SELESAI!"
echo "📍 Versi  : $next_tag"
echo "🚀 Status : Sentinel & Gateway aman di Awan."
