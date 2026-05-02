package snapshot

import (
    "archive/tar"
    "compress/gzip" // ✅ Ini yang benar
    "io"
    "os"
    "path/filepath"
    "strings"       // 🚩 Tambahkan ini juga karena kita pakai strings.TrimPrefix
)


func CreateSnapshot(src string, dest string) error {
    file, err := os.Create(dest)
    if err != nil {
        return err
    }
    defer file.Close()

    gw := gzip.NewWriter(file)
    defer gw.Close()

    tw := tar.NewWriter(gw)
    defer tw.Close()

    return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        header, err := tar.FileInfoHeader(info, info.Name())
        if err != nil { return err }
        
        header.Name = filepath.Join(filepath.Base(src), strings.TrimPrefix(path, src))
        if err := tw.WriteHeader(header); err != nil { return err }
        
        if !info.Mode().IsRegular() { return nil }
        
        f, err := os.Open(path)
        if err != nil { return err }
        defer f.Close()
        
        _, err = io.Copy(tw, f)
        return err
    })
}
