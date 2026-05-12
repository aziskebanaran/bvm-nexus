package user

import (
    "image"
    "image/color"
    "image/draw"
    "image/png"
    "net/http"
    "strings"
)

// ExecuteRender: Fungsi inti untuk mencetak visual aset
func ExecuteRender(w http.ResponseWriter, id string) {
    // 1. Buat Kanvas Kosong (300x400)
    img := image.NewRGBA(image.Rect(0, 0, 300, 400))

    // 2. Logika Warna Berdasarkan ID atau Tipe
    // Contoh: Jika ID mengandung 'REWARD', beri warna emas
    bgColor := color.RGBA{45, 52, 54, 255} // Default Dark
    if strings.Contains(id, "REWARD") {
        bgColor = color.RGBA{241, 196, 15, 255} // Gold
    } else if strings.Contains(id, "ASSET") {
        bgColor = color.RGBA{46, 204, 113, 255} // Green (Gacha)
    }

    // 3. Gambar Background
    draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

    // 4. Tambahkan Frame Dalam
    borderColor := color.RGBA{255, 255, 255, 50}
    draw.Draw(img, image.Rect(10, 10, 290, 390), &image.Uniform{borderColor}, image.Point{}, draw.Over)

    // 5. Kirim sebagai PNG
    w.Header().Set("Content-Type", "image/png")
    png.Encode(w, img)
}
