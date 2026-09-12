// Package ytdlp yt-dlp aracını arar, çalıştırır ve ilerlemeyi (progress) raporlar.
//
// yt-dlp sistemde PATH üzerinde olabileceği gibi uygulamanın araç dizininde de
// tutulabilir (tek tıkla kurulum). Binary, indirme sırasında ilerlemeyi
// "--newline" modunda stdout'a yazar; bu satırlardan yüzde çözülür.
package ytdlp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"syntaxoriginmultitools/internal/toolkit"
)

// ErrNotFound yt-dlp'nin bulunamadığını belirtir.
var ErrNotFound = errors.New("yt-dlp bulunamadı (Araçlar sekmesinden kurun)")

// YTDLP yt-dlp konumunu ve durumunu taşır.
type YTDLP struct {
	Dir  string // uygulamanın araç dizini (binary burada aranır)
	Path string // çözümlenmiş binary yolu; yoksa boş
	Err  error  // çözümleme hatası
}

// New bir yt-dlp oturumu kurar ve binary'yi çözer.
func New(dir string) *YTDLP {
	d := &YTDLP{Dir: dir}
	d.resolve()
	return d
}

func (d *YTDLP) resolve() {
	p, ok := toolkit.FindBin(d.Dir, "yt-dlp")
	if !ok {
		d.Path = ""
		d.Err = ErrNotFound
		return
	}
	d.Path = p
	d.Err = nil
}

// check binary'yi yeniden çözer (kurulum/güncelleme sonrası çağrılır).
func (d *YTDLP) check() error {
	path, _ := toolkit.FindBin(d.Dir, "yt-dlp")
	if path == "" {
		d.Path = ""
		d.Err = ErrNotFound
		return d.Err
	}
	d.Path = path
	d.Err = nil
	return nil
}

// Refresh binary'yi yeniden çözer ve sonucu döndürür (kurulum sonrası).
func (d *YTDLP) Refresh() error { return d.check() }

// Available yt-dlp'nin kullanılabilir olup olmadığını döndürür.
func (d *YTDLP) Available() bool { return d.check() == nil }

var (
	pctRe    = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
	threadRe = regexp.MustCompile(`^\s*\[`)
)

// Options bir indirme isteğini tanımlar.
type Options struct {
	// URL indirilecek platform adresi.
	URL string
	// OutDir çıktının yazılacağı dizin.
	OutDir string
	// Mode "video" veya "mp3" olabilir.
	Mode string
	// Quality video için çözünürlük: "best", "2160", "1440", "1080", "720", "480".
	Quality string
	// NoMerge video'da akışları birleştirmeden tek dosya olarak indirir
	// (ffmpeg yokken yararlıdır).
	NoMerge bool
}

// Download belirtilen URL'i indirir ve üretilen dosyanın yolunu döndürür.
// progress kanalına 0-100 arası değerler gönderilir; nil olabilir.
func (d *YTDLP) Download(ctx context.Context, o Options, progress chan<- float64) (string, error) {
	if err := d.check(); err != nil {
		return "", err
	}
	if o.URL == "" {
		return "", fmt.Errorf("URL boş olamaz")
	}
	if err := os.MkdirAll(o.OutDir, 0o755); err != nil {
		return "", fmt.Errorf("çıktı dizini oluşturulamadı: %w", err)
	}

	before := snapshotDir(o.OutDir)

	tmpl := filepath.Join(o.OutDir, "%(title)s [%(id)s]."+"%(ext)s")

	args := []string{"--newline", "--no-playlist", "--no-warnings", "--retries", "3"}
	switch o.Mode {
	case "mp3":
		args = append(args, "-f", "ba/b",
			"-x", "--audio-format", "mp3", "--audio-quality", "0",
			"-o", filepath.Join(o.OutDir, "%(title)s [%(id)s].mp3"))
	default:
		if o.NoMerge {
			args = append(args, "-f", "b", "-o", tmpl)
		} else {
			args = append(args, "-f", formatExpr(o.Quality),
				"--merge-output-format", "mp4",
				"-o", tmpl)
		}
	}
	args = append(args, o.URL)

	cmd := exec.CommandContext(ctx, d.Path, args...)
	toolkit.HideConsoleWindow(cmd)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("yt-dlp başlatılamadı: %w", err)
	}

	go func() {
		defer pw.Close()
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 256*1024), 256*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if progress == nil {
				continue
			}
			if m := pctRe.FindStringSubmatch(line); len(m) == 2 && !threadRe.MatchString(line) {
				p := parsePct(m[1])
				select {
				case progress <- p:
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()

	waitErr := cmd.Wait()
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return "", fmt.Errorf("yt-dlp indirme hatası: %w", waitErr)
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("yt-dlp hatası: %w", waitErr)
	}

	out := diffDir(o.OutDir, before)
	if out == "" {
		return "", fmt.Errorf("indirme tamamlandı ama çıktı dosyası bulunamadı")
	}
	return out, nil
}

// parsePct "45.2" gibi bir yüzde metnini 0-100 float'a çevirir.
func parsePct(s string) float64 {
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0
	}
	if v > 100 {
		v = 100
	}
	if v < 0 {
		v = 0
	}
	return v
}

// formatExpr seçilen çözünürlüğü bir yt-dlp format ifadesine çevirir.
func formatExpr(q string) string {
	switch q {
	case "480":
		return "bv*[height<=480]+ba/b[height<=480]/b"
	case "720":
		return "bv*[height<=720]+ba/b[height<=720]/b"
	case "1080":
		return "bv*[height<=1080]+ba/b[height<=1080]/b"
	case "1440":
		return "bv*[height<=1440]+ba/b[height<=1440]/b"
	case "2160":
		return "bv*[height<=2160]+ba/b[height<=2160]/b"
	default:
		return "bv*+ba/b"
	}
}

// snapshotDir dizindeki dosyaların yol kümesini döndürür.
func snapshotDir(dir string) map[string]bool {
	set := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return set
	}
	for _, e := range entries {
		if !e.IsDir() {
			set[filepath.Join(dir, e.Name())] = true
		}
	}
	return set
}

// diffDir iş sırasında YENİ oluşturulmuş en güncel dosyayı bulur.
func diffDir(dir string, before map[string]bool) string {
	after, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var newest string
	var newestMod time.Time
	for _, e := range after {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if before[p] {
			continue
		}
		info, _ := e.Info()
		if info == nil {
			continue
		}
		if info.ModTime().After(newestMod) && shouldKeep(e.Name()) {
			newest = p
			newestMod = info.ModTime()
		}
	}
	return newest
}

// shouldKeep bir dosyanın önbellek/parça artığı olup olmadığını denetler.
func shouldKeep(name string) bool {
	lower := strings.ToLower(name)
	skip := []string{".part", ".ytdl", ".fragments", ".m4a", ".f1", ".f2", ".f3", ".f4", ".f5", ".f6", ".f7", ".f8", ".f9", ".temp", ".download", ".mp4."}
	for _, s := range skip {
		if strings.Contains(lower, s) {
			return false
		}
	}
	return true
}
