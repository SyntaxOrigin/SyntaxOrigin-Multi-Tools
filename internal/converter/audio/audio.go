// Package audio MP3, WAV, FLAC, AAC, OGG arasında dönüştürme yapar.
package audio

import (
	"context"
	"fmt"

	"syntaxoriginmultitools/internal/toolkit"
)

// Supported hedef formatların listesi.
var Supported = []string{"mp3", "wav", "flac", "aac", "ogg"}

// CodecMap her hedef format için ffmpeg ses codec'sini tanımlar.
var CodecMap = map[string]string{
	"mp3":  "libmp3lame",
	"wav":  "pcm_s16le",
	"flac": "flac",
	"aac":  "aac",
	"ogg":  "libvorbis",
}

// Options dönüştürme seçenekleri.
type Options struct {
	Input  string
	Format string
	// Bitrate ses bit hızı (kbps). 0 ise varsayılan kullanılır.
	Bitrate int
}

// Convert girdiyi istenen ses formatına dönüştürür. Video girdiler senkronize
// ses akışı içeriyorsa da çıkarılır (ffmpeg otomatik seçer, -vn ile görüntü atlanır).
func Convert(ctx context.Context, o Options, progress chan<- float64) (string, error) {
	if o.Input == "" {
		return "", fmt.Errorf("girdi dosyası belirtilmedi")
	}
	codec, ok := CodecMap[o.Format]
	if !ok {
		return "", fmt.Errorf("desteklenmeyen ses formatı: %q (desteklenen: %v)", o.Format, Supported)
	}

	out, err := toolkit.OutputPath(o.Input, o.Format, 1)
	if err != nil {
		return "", err
	}

	args := []string{"-i", o.Input, "-vn"}
	if o.Bitrate > 0 {
		args = append(args, "-b:a", fmt.Sprintf("%dk", o.Bitrate))
	}
	args = append(args, "-c:a", codec, out)

	if err := toolkit.RunFFmpegWithProgress(ctx, args, progress); err != nil {
		return "", err
	}
	return out, nil
}
