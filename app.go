// Arayüz bindings'leri: Wails runtime üzerinden ön yüze çağrılan tüm metodlar burada.
// İşler (convert/download/install) goroutine'de koşar, ilerleme "job" olayıyla
// ön yüze yayınlanır.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"syntaxoriginmultitools/internal/converter/audio"
	"syntaxoriginmultitools/internal/converter/document"
	"syntaxoriginmultitools/internal/converter/image"
	"syntaxoriginmultitools/internal/converter/video"
	"syntaxoriginmultitools/internal/tools"
	"syntaxoriginmultitools/internal/ytdlp"
)

// App uygulama durumunu taşır ve Wails'e bağlanır.
type App struct {
	ctx      context.Context
	tmpDir   string
	toolsDir string
	dlDir    string
	dl       *ytdlp.YTDLP
	seq      atomic.Int64
	lang     string
}

// NewApp yeni bir uygulama örneği kurar.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := os.UserConfigDir()
	if err != nil {
		cfg = os.TempDir()
	}
	a.toolsDir = filepath.Join(cfg, "SyntaxOrigin Multi Tools", "bin")
	_ = os.MkdirAll(a.toolsDir, 0o755)

	a.tmpDir, _ = os.MkdirTemp("", "syntaxorigin_*")
	a.dlDir = defaultDownloadDir()
	a.dl = ytdlp.New(a.toolsDir)
	a.lang = "tr"
}

func (a *App) shutdown(_ context.Context) {
	if a.tmpDir != "" {
		_ = os.RemoveAll(a.tmpDir)
	}
}

// ---- Olay yayını ----

// JobEvent ön yüze yayınlanan iş durum olayıdır.
type JobEvent struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"` // convert | download | install
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	Output   string  `json:"output"`
}

func (a *App) emit(e JobEvent) { wruntime.EventsEmit(a.ctx, "job", e) }

// runJob bir işi arka planda başlatır ve iş kimliğini döndürür.
func (a *App) runJob(kind string, fn func(chan<- float64, func(string)) (string, error)) string {
	id := "job_" + strconv.FormatInt(a.seq.Add(1), 10)
	a.emit(JobEvent{ID: id, Kind: kind, State: "running", Progress: 0, Message: "@msg.ready"})

	go func() {
		prog := make(chan float64)
		progDone := make(chan struct{})
		go func() {
			for p := range prog {
				a.emit(JobEvent{ID: id, Kind: kind, State: "running", Progress: p})
			}
			close(progDone)
		}()

		out, err := fn(prog, func(m string) {
			a.emit(JobEvent{ID: id, Kind: kind, State: "running", Message: m})
		})
		close(prog)
		<-progDone

		if err != nil {
			a.emit(JobEvent{ID: id, Kind: kind, State: "error", Message: err.Error()})
			return
		}
		a.emit(JobEvent{ID: id, Kind: kind, State: "done", Progress: 100, Message: "@msg.done", Output: out})
	}()
	return id
}

// ---- Sabitler / bilgi ----

// AboutInfo Hakkında ekranında gösterilen bilgilerdir.
type AboutInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	GitHub  string `json:"github"`
	OrgURL  string `json:"orgUrl"`
	RepoURL string `json:"repoUrl"`
	Avatar  string `json:"avatar"`
	GoVer   string `json:"goVer"`
}

// About uygulama künyesini döndürür.
func (a *App) About() AboutInfo {
	return AboutInfo{
		Name:    appName,
		Version: appVersion,
		GitHub:  "SyntaxOrigin",
		OrgURL:  repoURL,
		RepoURL: repoURL,
		Avatar:  "https://github.com/SyntaxOrigin.png?size=64",
		GoVer:   runtime.Version(),
	}
}

// OpenExternal URL'i varsayılan tarayıcıda açar.
func (a *App) OpenExternal(url string) {
	if url == "" {
		return
	}
	wruntime.BrowserOpenURL(a.ctx, url)
}

// OpenFolder verilen yolu dosya yöneticisinde açar (dosya ise seçili gösterir).
func (a *App) OpenFolder(path string) {
	if path == "" {
		return
	}
	path = filepath.Clean(path)
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("explorer", "/select,", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}

// ---- Araç durumu ve kurulum ----

// Tools tüm harici araçların durumunu döndürür.
func (a *App) Tools() []tools.Tool { return tools.CheckAll(a.toolsDir) }

// InstallTool bir aracı tek tıkla kurar; iş kimliği döndürür (job events).
func (a *App) InstallTool(name string) string {
	return a.runJob("install", func(prog chan<- float64, setMsg func(string)) (string, error) {
		if err := tools.Install(a.ctx, a.toolsDir, name, prog, setMsg); err != nil {
			return "", err
		}
		_ = a.dl.Refresh()
		return a.toolsDir, nil
	})
}

// ---- Format listesi ----

// FormatOption bir hedef format seçeneğidir.
type FormatOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func opts(values []string) []FormatOption {
	out := make([]FormatOption, 0, len(values))
	for _, v := range values {
		out = append(out, FormatOption{Value: v, Label: strings.ToUpper(v)})
	}
	return out
}

// Formats tüm dönüştürme türleri için hedef format listelerini döndürür.
func (a *App) Formats() map[string][]FormatOption {
	docs := make([]FormatOption, 0, len(document.Supported))
	for _, s := range document.Supported {
		docs = append(docs, FormatOption{Value: s.Key, Label: s.Desc})
	}
	return map[string][]FormatOption{
		"video":    opts(video.Supported),
		"audio":    opts(audio.Supported),
		"image":    opts(image.Supported),
		"document": docs,
	}
}

// ---- Dosya / klasör seçimleri ----

// PickInputFile dönüştürülecek dosyayı seçtirir.
func (a *App) PickInputFile() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: a.tr("dlg.pickInput"),
		Filters: []wruntime.FileFilter{
			{DisplayName: a.tr("dlg.filterSupported"), Pattern: "*.mp4;*.mkv;*.avi;*.mov;*.webm;*.mp3;*.wav;*.flac;*.aac;*.ogg;*.png;*.jpg;*.jpeg;*.webp;*.bmp;*.pdf;*.docx"},
			{DisplayName: a.tr("dlg.filterAll"), Pattern: "*.*"},
		},
	})
}

// PickOutputFile çıktı konumunu seçtirir; önerilen dosya adı yerleşiktir.
func (a *App) PickOutputFile(input, kind, format string) (string, error) {
	dir := ""
	if input != "" {
		dir = filepath.Dir(input)
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:                a.tr("dlg.pickOutput"),
		DefaultDirectory:     dir,
		DefaultFilename:      suggestedName(input, kind, format),
		CanCreateDirectories: true,
	})
}

func suggestedName(input, kind, format string) string {
	base := "output"
	if input != "" {
		base = strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	}
	ext := format
	if kind == "document" {
		switch format {
		case "pdf_to_image":
			ext = "png"
		case "pdf_to_text":
			ext = "txt"
		case "docx_to_pdf":
			ext = "pdf"
		case "pdf_to_docx":
			ext = "docx"
		}
	}
	return base + "." + ext
}

// --- İndirme ---

// DownloadRequest bir platform indirme isteğidir.
type DownloadRequest struct {
	URL     string `json:"url"`
	Mode    string `json:"mode"`    // video | mp3
	Quality string `json:"quality"` // best | 2160 | 1440 | 1080 | 720 | 480
	OutDir  string `json:"outDir"`
}

// Download platformdan video/müzik indirir; iş kimliği döndürür.
func (a *App) Download(req DownloadRequest) string {
	if req.OutDir == "" {
		req.OutDir = a.dlDir
	}
	ffmpegOK := tools.ToolFound(a.toolsDir, "ffmpeg")
	return a.runJob("download", func(prog chan<- float64, setMsg func(string)) (string, error) {
		if err := a.dl.Refresh(); err != nil {
			return "", errors.New("@err.ytdlp")
		}
		setMsg("@msg.resolving")
		return a.dl.Download(a.ctx, ytdlp.Options{
			URL:     req.URL,
			OutDir:  req.OutDir,
			Mode:    req.Mode,
			Quality: req.Quality,
			NoMerge: req.Mode != "mp3" && !ffmpegOK,
		}, prog)
	})
}

// --- Dönüştürme ---

// ConvertRequest bir dönüştürme isteğidir.
type ConvertRequest struct {
	Input      string `json:"input"`
	Kind       string `json:"kind"`
	Format     string `json:"format"`
	Bitrate    int    `json:"bitrate"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	OutputPath string `json:"outputPath"`
}

// Convert bir dosyayı istenen formata çevirir; iş kimliği döndürür.
func (a *App) Convert(req ConvertRequest) string {
	return a.runJob("convert", func(prog chan<- float64, setMsg func(string)) (string, error) {
		if req.Input == "" {
			return "", errors.New("kaynak dosya seçilmedi")
		}
		if req.OutputPath == "" {
			return "", errors.New("çıktı konumu seçilmedi")
		}
		outDir := filepath.Dir(req.OutputPath)
		if _, err := os.Stat(outDir); err != nil {
			return "", fmt.Errorf("çıktı dizini geçersiz: %w", err)
		}

		// Kaynağı çıktı dizinine kopyala; dönüştürücüler çıktıyı girdinin yanına
		// yazar (kopya X→ hedef). Kopyalama aşaması ilerlemenin %0-20'sini kaplar.
		setMsg("@msg.copySource")
		tmpIn, err := copyInput(req.Input, outDir, func(p float64) { prog <- p * 0.2 })
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpIn)

		if req.Kind == "document" {
			setMsg("@msg.converting")
			out, err := document.ConvertKey(a.ctx, req.Format, tmpIn, nil)
			if err != nil {
				return "", err
			}
			return finalizeOutput(out, req.OutputPath)
		}

		// Dönüştürücü ilerlemesi %20-100 aralığına taşınır.
		conv := make(chan float64)
		convDone := make(chan struct{})
		go func() {
			for p := range conv {
				prog <- 20 + p*0.8
			}
			close(convDone)
		}()

		var out string
		var convertErr error
		switch req.Kind {
		case "video":
			out, convertErr = video.Convert(a.ctx, video.Options{Input: tmpIn, Format: req.Format, Bitrate: req.Bitrate}, conv)
		case "audio":
			out, convertErr = audio.Convert(a.ctx, audio.Options{Input: tmpIn, Format: req.Format, Bitrate: req.Bitrate}, conv)
		case "image":
			out, convertErr = image.Convert(a.ctx, image.Options{Input: tmpIn, Format: req.Format, Width: req.Width, Height: req.Height}, conv)
		default:
			close(conv)
			return "", fmt.Errorf("bilinmeyen dönüştürme türü: %q", req.Kind)
		}
		close(conv)
		<-convDone

		if convertErr != nil {
			return "", convertErr
		}
		return finalizeOutput(out, req.OutputPath)
	})
}

// finalizeOutput üretilen dosyayı kullanıcının seçtiği çıktı yoluna taşır.
// Farklı disk/sürücü adı durumunda (os.Rename başarısız olur) kopyalama yapılır.
func finalizeOutput(generated, target string) (string, error) {
	if generated == "" {
		return "", errors.New("dönüştürme çıktı üretmedi")
	}
	if generated == target {
		return target, nil
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("mevcut çıktı silinemedi: %w", err)
	}
	if err := os.Rename(generated, target); err != nil {
		if cerr := copyOverride(generated, target); cerr != nil {
			return "", fmt.Errorf("çıktı kaydedilemedi: %w (taşıma hatası: %v)", cerr, err)
		}
	}
	return target, nil
}

// copyOverride bir dosyanın içeriğini hedefe kopyalar (cross-device yardımı).
func copyOverride(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

// copyInput kaynak dosyayı hedef dizine yüzdelik ilerlemeyle kopyalar.
func copyInput(src, dir string, progress func(float64)) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("kaynak açılamadı: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, "syntaxorigin_input_*"+filepath.Ext(src))
	if err != nil {
		return "", fmt.Errorf("geçici dosya oluşturulamadı: %w", err)
	}
	name := tmp.Name()

	buf := make([]byte, 1<<20)
	var written int64
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				_ = tmp.Close()
				_ = os.Remove(name)
				return "", werr
			}
			written += int64(n)
			if progress != nil && info.Size() > 0 {
				progress(float64(written) / float64(info.Size()) * 100)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			_ = tmp.Close()
			_ = os.Remove(name)
			return "", rerr
		}
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// defaultDownloadDir varsayılan indirme klasörünü bulur.
func defaultDownloadDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dl := filepath.Join(home, "Downloads")
		if info, err := os.Stat(dl); err == nil && info.IsDir() {
			return dl
		}
		return home
	}
	return os.TempDir()
}

// DownloadDir geçerli indirme klasörünü döndürür.
func (a *App) DownloadDir() string { return a.dlDir }

// PickDownloadDir indirme klasörünü seçtirir; seçilirse varsayılanı günceller.
func (a *App) PickDownloadDir() (string, error) {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: a.tr("dlg.pickOutput"),
	})
	if err != nil {
		return "", err
	}
	if dir != "" {
		a.dlDir = dir
	}
	return dir, nil
}
