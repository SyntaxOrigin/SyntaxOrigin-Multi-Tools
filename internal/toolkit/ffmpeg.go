// Package toolkit ffmpeg ve diğer harici CLI araçlarını güvenli biçimde çağırır,
// eksik bağımlılıkları tespit eder ve ilerlemeyi (progress) raporlar.
package toolkit

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DepNotFound bir harici aracın sistemde bulunamadığını belirtir.
var ErrCommandNotFound = errors.New("harici araç bulunamadı")

// LookPath bir aracın sistemde kurulu olup olmadığını kontrol eder.
func LookPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %q (lütfen kurun ve PATH'e ekleyin)", ErrCommandNotFound, name)
	}
	return p, nil
}

var (
	durationRe = regexp.MustCompile(`^duration\s*=\s*(\d+)$`)
	timeRe     = regexp.MustCompile(`^out_time_ms\s*=\s*(-?\d+)$`)
	progressRe = regexp.MustCompile(`^progress\s*=\s*(continue|end)$`)
)

// runFFmpegWithProgress ffmpeg'i "-progress pipe:1 -nostats" modunda çalıştırır,
// çıktıdan yüzde hesaplar ve progress kanalına gönderir.
// progress nil olabilir (ilerleme takibi istenmiyorsa).
func RunFFmpegWithProgress(ctx context.Context, args []string, progress chan<- float64) error {
	bin, err := LookPath("ffmpeg")
	if err != nil {
		return err
	}

	fullArgs := append([]string{"-hide_banner", "-nostats", "-y", "-progress", "pipe:1"}, args...)
	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	HideConsoleWindow(cmd)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout kanalı açılamadı: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg başlatılamadı: %w", err)
	}

	// Duration bilinene kadar ilerleme raporlama. Video/audio için ffmpeg
	// "-progress" çıktısında duration yayınlar; image dönüşümlerinde ise
	// genelde tek kare olduğundan anında biter.
	var total, last int64
	s := bufio.NewScanner(stdout)
	s.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if progress == nil {
			continue
		}
		if m := durationRe.FindStringSubmatch(line); m != nil {
			total, _ = strconv.ParseInt(m[1], 10, 64)
			continue
		}
		if m := timeRe.FindStringSubmatch(line); m != nil {
			now, _ := strconv.ParseInt(m[1], 10, 64)
			if now > last {
				last = now
			}
			if total > 0 {
				pct := float64(last) / float64(total) * 100
				if pct > 100 {
					pct = 100
				}
				select {
				case progress <- pct:
				case <-ctx.Done():
					_ = cmd.Process.Kill()
					return ctx.Err()
				default:
				}
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg hatası: %w", err)
	}
	return nil
}

// EnsureExt gerekli değilse dosya uzantısını ekler.
func EnsureExt(path, ext string) string {
	if strings.EqualFold(filepath.Ext(path), "."+ext) {
		return path
	}
	return path + "." + ext
}

// OutputPath verilen girdi dosyasının bulunduğu dizine yeni uzantıyla bir çıktı yolu üretir.
func OutputPath(input, newExt string, index int) (string, error) {
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(abs)
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	suffix := ""
	if index > 1 {
		suffix = fmt.Sprintf("_%d", index)
	}
	return filepath.Join(dir, base+suffix+"."+newExt), nil
}

// ExeExt platforma göre çalıştırılabilir uzantısını döndürür.
func ExeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// FindBin önce dir içinde, bulunamazsa sistem PATH'inde araç arar.
func FindBin(dir, name string) (string, bool) {
	for _, n := range []string{name, name + ExeExt()} {
		if dir != "" {
			p := filepath.Join(dir, n)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, true
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, true
	}
	return "", false
}

// ToolVersion aracın "-version" veya "--version" çıktısının ilk satırını döndürür.
func ToolVersion(bin string) string {
	for _, flag := range []string{"--version", "-version"} {
		cmd := exec.Command(bin, flag)
		HideConsoleWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		if line != "" {
			return line
		}
	}
	return ""
}

// DownloadFile bir URL'i hedef yola indirir; ilerlemeyi (0-100) progress kanalına gönderir.
func DownloadFile(ctx context.Context, url, dest string, progress chan<- float64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("istek kurulamadı: %w", err)
	}
	req.Header.Set("User-Agent", "SyntaxOrigin-MultiTools/2.0")

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("indirme başarısız: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("indirme hatası (HTTP %d): %s", resp.StatusCode, resp.Status)
	}

	out := dest + ".part"
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("dosya oluşturulamadı: %w", err)
	}
	defer f.Close()

	total := resp.ContentLength
	buf := make([]byte, 256*1024)
	var written int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return fmt.Errorf("yazma hatası: %w", werr)
			}
			written += int64(n)
			if progress != nil && total > 0 {
				pct := float64(written) / float64(total) * 100
				select {
				case progress <- pct:
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return fmt.Errorf("okuma hatası: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(out, dest); err != nil {
		_ = os.Remove(out)
		return fmt.Errorf("dosya adlandırılamadı: %w", err)
	}
	return nil
}
