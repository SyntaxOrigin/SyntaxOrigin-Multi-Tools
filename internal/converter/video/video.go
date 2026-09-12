// Package video MP4, MKV, AVI, MOV ve WebM arasında dönüştürme yapar.
package video

import (
	"context"
	"fmt"

	"syntaxoriginmultitools/internal/toolkit"
)

// Supported hedef formatların listesi (arayüzde gösterim için).
var Supported = []string{"mp4", "mkv", "avi", "mov", "webm"}

// Options dönüştürme seçeneklerini taşır.
type Options struct {
	// Input kaynak dosya yolu. Boş bırakılırsa format seçimi için stream içeriği şart.
	Input string
	// Format hedef uzantı (mp4, mkv, ...). Yalnızca bu geçerli değerlere izin verilir.
	Format string
	// Bitrate video bit hızı (kbps). 0 ise varsayılan korunur.
	Bitrate int
}

// Convert girdi dosyasını istenen formata dönüştürür.
func Convert(ctx context.Context, o Options, progress chan<- float64) (string, error) {
	if o.Input == "" {
		return "", fmt.Errorf("girdi dosyası belirtilmedi")
	}
	if !valid(o.Format) {
		return "", fmt.Errorf("desteklenmeyen video formatı: %q (desteklenen: %v)", o.Format, Supported)
	}

	out, err := toolkit.OutputPath(o.Input, o.Format, 1)
	if err != nil {
		return "", err
	}

	args := []string{"-i", o.Input}
	if o.Bitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", o.Bitrate))
	}

	// Format-a-appropriate codec seçimi
	vcodec, acodec := codecForFormat(o.Format)
	args = append(args, "-c:v", vcodec, "-preset", "medium", "-c:a", acodec, out)

	if err := toolkit.RunFFmpegWithProgress(ctx, args, progress); err != nil {
		return "", err
	}
	return out, nil
}

// codecForFormat hedef formata göre video ve ses codec döndürür.
func codecForFormat(format string) (vcodec, acodec string) {
	switch format {
	case "webm":
		return "libvpx-vp9", "libopus" // WebM: VP9 + Opus (modern, yaygın destek)
	case "mp4", "mov":
		return "libx264", "aac" // MP4/MOV: H.264 + AAC (en yaygın uyumluluk)
	case "mkv":
		return "libx264", "aac" // MKV: H.264 + AAC (geniş destek)
	case "avi":
		return "libx264", "aac" // AVI: H.264 + AAC (bazı eski oynatıcılar için MPEG4 daha iyi ama bu yeterli)
	default:
		return "libx264", "aac"
	}
}

func valid(f string) bool {
	for _, s := range Supported {
		if s == f {
			return true
		}
	}
	return false
}
