// Package tools dönüştürme/indirme için gereken harici araçları denetler ve
// eksik olanları tek tıkla indirip hazırlar (ffmpeg, yt-dlp).
//
// LibreOffice ve poppler boyutları nedeniyle otomatik kurulamaz; bunlar için
// kullanıcı resmî indirme sayfasına yönlendirilir (GuideURL).
package tools

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"syntaxoriginmultitools/internal/toolkit"
)

// ErrUnsupported otomatik kurulumun bu araç için mümkün olmadığını belirtir.
var ErrUnsupported = errors.New("bu araç için otomatik kurulum yok")

// Tool bir harici aracın kurulum ve durum bilgisini taşır.
type Tool struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Found       bool   `json:"found"`
	Path        string `json:"path"`
	Version     string `json:"version"`
	Installable bool   `json:"installable"`
	Source      string `json:"source"`
	GuideURL    string `json:"guideUrl"`
}

// CheckAll tüm araçların sistemdeki durumunu döndürür.
func CheckAll(toolsDir string) []Tool {
	var out []Tool

	ff := Tool{
		Key: "ffmpeg", Name: "FFmpeg", Category: "Dönüştürme",
		Installable: true, GuideURL: "https://ffmpeg.org/download.html",
		Source: "Video, ses ve görüntü dönüşümlerinin temel motoru.",
	}
	if p, ok := toolkit.FindBin(toolsDir, "ffmpeg"); ok {
		ff.Found, ff.Path, ff.Version = true, p, toolkit.ToolVersion(p)
	}
	out = append(out, ff)

	dp := Tool{
		Key: "yt-dlp", Name: "yt-dlp", Category: "Platform İndirme",
		Installable: true, GuideURL: "https://github.com/yt-dlp/yt-dlp#installation",
		Source: "Binlerce platformdan (YouTube, TikTok, vb.) video/müzik indirir.",
	}
	if p, ok := toolkit.FindBin(toolsDir, "yt-dlp"); ok {
		dp.Found, dp.Path, dp.Version = true, p, toolkit.ToolVersion(p)
	}
	out = append(out, dp)

	lo := Tool{
		Key: "libreoffice", Name: "LibreOffice", Category: "Ofis / PDF",
		Installable: false, GuideURL: "https://www.libreoffice.org/download/download-libreoffice/",
		Source: "DOCX ↔ PDF dönüşümleri için gereklidir.",
	}
	for _, n := range []string{"libreoffice", "soffice"} {
		if p, err := exec.LookPath(n); err == nil {
			lo.Found, lo.Path = true, p
			break
		}
	}
	if lo.Found {
		lo.Version = toolkit.ToolVersion(lo.Path)
	}
	out = append(out, lo)

	po := Tool{
		Key: "poppler", Name: "poppler (PDF Araçları)", Category: "PDF",
		Installable: false, GuideURL: "https://poppler.freedesktop.org/",
		Source: "pdftoppm + pdftotext: PDF'ten görsel/metin çıkarma.",
	}
	var poPaths []string
	for _, n := range []string{"pdftoppm", "pdftotext"} {
		if p, err := exec.LookPath(n); err == nil {
			po.Found = true
			poPaths = append(poPaths, p)
		}
	}
	if po.Found {
		po.Path = strings.Join(poPaths, "; ")
		if len(poPaths) > 0 {
			po.Version = toolkit.ToolVersion(poPaths[0])
		}
	}
	out = append(out, po)

	return out
}

// toolExists bir aracın (gelgit durumlarına karşı) yeniden çözümlemesini yapar.
func toolExists(toolsDir, key string) bool {
	switch key {
	case "ffmpeg":
		_, ok := toolkit.FindBin(toolsDir, "ffmpeg")
		return ok
	case "yt-dlp":
		_, ok := toolkit.FindBin(toolsDir, "yt-dlp")
		return ok
	}
	return false
}

// Install aracı tek tıkla indirip kurar. progress 0-100, setMsg ise durum mesajı
// iletir (ikisi de nil olabilir).
func Install(ctx context.Context, toolsDir, key string, progress chan<- float64, setMsg func(string)) error {
	if setMsg == nil {
		setMsg = func(string) {}
	}
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		return fmt.Errorf("araç dizini oluşturulamadı: %w", err)
	}

	switch key {
	case "ffmpeg":
		if runtime.GOOS == "windows" {
			return installFFmpegWindows(ctx, toolsDir, progress, setMsg)
		}
		return installFFmpegLinux(ctx, toolsDir, progress, setMsg)
	case "yt-dlp":
		return installYtdlp(ctx, toolsDir, progress, setMsg)
	}
	return ErrUnsupported
}

// ffmpegDownloadURL platforma uygun ffmpeg statik paketini döndürür.
func ffmpegDownloadURL() string {
	if runtime.GOOS == "windows" {
		return "https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-win64-gpl.zip"
	}
	return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
}

// ytdlpDownloadURL platforma uygun yt-dlp binary adresini döndürür.
func ytdlpDownloadURL() string {
	switch runtime.GOOS {
	case "windows":
		return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	case "darwin":
		return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
	default:
		return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"
	}
}

func installYtdlp(ctx context.Context, toolsDir string, progress chan<- float64, setMsg func(string)) error {
	setMsg("@install.ytdlp")
	name := "yt-dlp" + toolkit.ExeExt()
	dest := filepath.Join(toolsDir, name)
	if err := toolkit.DownloadFile(ctx, ytdlpDownloadURL(), dest, progress); err != nil {
		return fmt.Errorf("yt-dlp kurulamadı: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dest, 0o755)
	}
	return nil
}

// installFFmpegWindows BtbN zip gönderisini indirir, içinden ffmpeg.exe/ffprobe.exe
// çıkarıp araç dizinine yerleştirir.
func installFFmpegWindows(ctx context.Context, toolsDir string, progress chan<- float64, setMsg func(string)) error {
	setMsg("@install.ffmpegWin")
	zipPath := filepath.Join(toolsDir, "ffmpeg.zip")
	if err := toolkit.DownloadFile(ctx, ffmpegDownloadURL(), zipPath, progress); err != nil {
		return fmt.Errorf("FFmpeg indirilemedi: %w", err)
	}
	defer os.Remove(zipPath)

	setMsg("@install.ffmpegExtract")
	if err := extractExeFromZip(zipPath, toolsDir, "ffmpeg"); err != nil {
		return fmt.Errorf("FFmpeg paketten çıkarılamadı: %w", err)
	}
	if err := extractExeFromZip(zipPath, toolsDir, "ffprobe"); err != nil {
		// ffprobe zorunlu değil; hata normal işleyişi bozmasın
		_ = err
	}
	return nil
}

// extractExeFromZip zip'teki belirtilen adlı (ffmpeg.exe vb.) ilk dosyayı
// toolsDir içine kopyalar. Büyük onarımlı paketlerde birden çok eşleşme
// olabileceğinden son eşleşen örnek alınır.
func extractExeFromZip(zipPath, destDir, name string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	target := name + toolkit.ExeExt()
	var src *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Base(f.Name), target) {
			src = f
		}
	}
	if src == nil {
		return fmt.Errorf("zip içinde %q bulunamadı", target)
	}

	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(filepath.Join(destDir, target))
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		_ = os.Remove(out.Name())
		return err
	}
	return out.Close()
}

// installFFmpegLinux johnvansickle statik paketini indirir, tar ile açar ve
// ffmpeg/ffprobe/ffplay ikililerini araç dizinine kopyalar.
func installFFmpegLinux(ctx context.Context, toolsDir string, progress chan<- float64, setMsg func(string)) error {
	setMsg("@install.ffmpegLinux")
	arc := filepath.Join(toolsDir, "ffmpeg.tar.xz")
	if err := toolkit.DownloadFile(ctx, ffmpegDownloadURL(), arc, progress); err != nil {
		return fmt.Errorf("FFmpeg indirilemedi: %w", err)
	}
	defer os.Remove(arc)

	setMsg("@install.ffmpegExtract")
	work, err := os.MkdirTemp("", "syntaxorigin_ffmpeg_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	cmd := exec.CommandContext(ctx, "tar", "-xJf", arc, "-C", work)
	toolkit.HideConsoleWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar hatası: %s: %w", strings.TrimSpace(string(out)), err)
	}

	copied := false
	err = filepath.Walk(work, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		base := filepath.Base(p)
		if base == "ffmpeg" || base == "ffprobe" || base == "ffplay" {
			dest := filepath.Join(toolsDir, base)
			if cerr := copyFile(p, dest); cerr != nil {
				return cerr
			}
			_ = os.Chmod(dest, 0o755)
			copied = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !copied {
		return fmt.Errorf("paket içinde ffmpeg ikilisi bulunamadı")
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// ToolFound düşük seviyeli bir araç varlığı kontrolüdür (App'in kurulum sonrası
// durumu yenilemesi için).
func ToolFound(toolsDir, key string) bool { return toolExists(toolsDir, key) }
