// Package image PNG, JPEG, WebP, BMP arasında dönüştürme ve boyutlandırma yapar.
//
// WebP ve BMP "encode" desteği Go standart image paketinde bulunmadığından ve
// "sıfır harici Go kütüphanesi" kısıtı geçerli olduğundan, dönüşümler ffmpeg
// üzerinden gerçekleştirilir (ffmpeg her platformda kurulabilir).
package image

import (
	"context"
	"fmt"

	"syntaxoriginmultitools/internal/toolkit"
)

// Supported hedef formatların listesi.
var Supported = []string{"png", "jpg", "webp", "bmp"}

// Options dönüştürme seçenekleri.
type Options struct {
	Input  string
	Format string
	// Width/Height tek tek 0 sa değer korunur. İkisi de verilirse oran bozulabilir;
	// yalnızca biri verilirse oran korunur.
	Width  int
	Height int
}

// Convert girdi görüntüsünü istenen formata çevirir; boyut verildiyse ölçekler.
func Convert(ctx context.Context, o Options, progress chan<- float64) (string, error) {
	if o.Input == "" {
		return "", fmt.Errorf("girdi dosyası belirtilmedi")
	}
	if !valid(o.Format) {
		return "", fmt.Errorf("desteklenmeyen görüntü formatı: %q (desteklenen: %v)", o.Format, Supported)
	}

	out, err := toolkit.OutputPath(o.Input, o.Format, 1)
	if err != nil {
		return "", err
	}

	args := []string{"-i", o.Input}
	if o.Width > 0 || o.Height > 0 {
		scale := scaleExpr(o)
		args = append(args, "-vf", scale)
	}
	args = append(args, out)

	// Görüntü dönüşümleri tek kare olduğundan progress takibi isteğe bağlı.
	if err := toolkit.RunFFmpegWithProgress(ctx, args, progress); err != nil {
		return "", err
	}
	return out, nil
}

// scaleExpr ffmpeg için bir scale ifadesi üretir. Yalnızca bir kenar verilirse
// -1 kullanarak en-boy oranını korur.
func scaleExpr(o Options) string {
	switch {
	case o.Width > 0 && o.Height > 0:
		return fmt.Sprintf("scale=%d:%d", o.Width, o.Height)
	case o.Width > 0:
		return fmt.Sprintf("scale=%d:-1", o.Width)
	case o.Height > 0:
		return fmt.Sprintf("scale=-1:%d", o.Height)
	}
	return "scale=-1:-1"
}

func valid(f string) bool {
	for _, s := range Supported {
		if s == f {
			return true
		}
	}
	return false
}
