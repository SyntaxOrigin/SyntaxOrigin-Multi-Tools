// Package document PDF'ten görsel/metin ve Word(DOCX) - PDF dönüşümleri yapar.
//
// Harici GUI araçları (LibreOffice) kullanıldığı için eksik olmaları durumunda
// kullanıcı net biçimde uyarılır.
package document

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"syntaxoriginmultitools/internal/toolkit"
)

var (
	// ErrLibreOffice LibreOffice'in kurulu olmadığını belirtir.
	ErrLibreOffice = errors.New("LibreOffice bulunamadı; DOCX-PDF dönüşümü için gerekli")
	// ErrPoppler poppler araçlarının (pdftoppm/pdftotext) eksik olduğunu belirtir.
	ErrPoppler = errors.New("poppler araçları bulunamadı; PDF görsel/metin dönüşümü için gerekli")
)

// Supported dönüşüm tiplerinin listesi.
var Supported = []struct {
	Key  string
	Desc string
}{
	{"pdf_to_image", "PDF → Görsel (PNG)"},
	{"pdf_to_text", "PDF → Metin (TXT)"},
	{"docx_to_pdf", "Word (DOCX) → PDF"},
	{"pdf_to_docx", "PDF → Word (DOCX)"},
}

// ConvertKey belirtilen anahtar için dönüşümü gerçekleştirir.
func ConvertKey(ctx context.Context, key, input string, progress chan<- float64) (string, error) {
	switch key {
	case "pdf_to_image":
		return pdfToImage(ctx, input)
	case "pdf_to_text":
		return pdfToText(input)
	case "docx_to_pdf":
		return officeToPDF(ctx, input)
	case "pdf_to_docx":
		return pdfToDocx(ctx, input)
	}
	return "", fmt.Errorf("bilinmeyen doküman dönüşümü: %q", key)
}

// pdfToImage her sayfayı ayrı bir PNG'a çevirir; ilk sayfayı üretir ve progres
// olarak sayfa sayısı raporlanır. Şimdilik tek sayfa PNG döner.
func pdfToImage(ctx context.Context, input string) (string, error) {
	bin, err := toolkit.LookPath("pdftoppm")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPoppler, err)
	}
	pdf, err := toolkit.OutputPath(input, "png", 1)
	if err != nil {
		return "", err
	}
	outBase := strings.TrimSuffix(pdf, ".png")

	args := []string{"-png", "-r", "150", input, outBase}
	cmd := exec.CommandContext(ctx, bin, args...)
	toolkit.HideConsoleWindow(cmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftoppm hatası: %w", err)
	}
	// "-png + -r" ile üretilen dosya "outBase-1.png" biçimindedir.
	return outBase + "-1.png", nil
}

// pdfToText PDF'in metnini TXT dosyasına çıkarır.
func pdfToText(input string) (string, error) {
	bin, err := toolkit.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPoppler, err)
	}
	out, err := toolkit.OutputPath(input, "txt", 1)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(bin, input, out)
	toolkit.HideConsoleWindow(cmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext hatası: %w", err)
	}
	return out, nil
}

// officeToPDF LibreOffice'i headless modda çalıştırarak (DOCX dahil) bir ofis
// dosyasını PDF'e dönüştürür.
func officeToPDF(ctx context.Context, input string) (string, error) {
	bin, err := toolkit.LookPath("libreoffice")
	if err != nil {
		// Windows'ta "soffice.exe" adıyla da bulunabilir.
		bin, err = toolkit.LookPath("soffice")
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrLibreOffice, err)
		}
	}
	dir := filepath.Dir(input)

	args := []string{"--headless", "--convert-to", "pdf", "--outdir", dir, input}
	cmd := exec.CommandContext(ctx, bin, args...)
	toolkit.HideConsoleWindow(cmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("LibreOffice hatası: %w", err)
	}
	out := strings.TrimSuffix(input, filepath.Ext(input)) + ".pdf"
	return out, nil
}

// pdfToDocx PDF'i LibreOffice üzerinden DOCX'e dönüştürür (PDF Import).
func pdfToDocx(ctx context.Context, input string) (string, error) {
	bin, err := toolkit.LookPath("libreoffice")
	if err != nil {
		bin, err = toolkit.LookPath("soffice")
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrLibreOffice, err)
		}
	}
	dir := filepath.Dir(input)
	args := []string{"--headless", "--convert-to", "docx", "--outdir", dir, input}
	cmd := exec.CommandContext(ctx, bin, args...)
	toolkit.HideConsoleWindow(cmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("LibreOffice (PDF→DOCX) hatası: %w", err)
	}
	return strings.TrimSuffix(input, filepath.Ext(input)) + ".docx", nil
}
