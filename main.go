// SyntaxOrigin Multi Tools — çoklu dosya formatı dönüştürücü + platform indirici.
//
// Window'lar ve Linux masaüstünde çalışan gerçek bir masaüstü uygulamasıdır
// (Wails v2; arayüz uygulamanın kendi penceresinde işlenir, tarayıcı yok).
//
//	wails dev   → geliştirme modu (hızlı yeniden yükleme)
//	wails build → üretim ikilisi (build/bin/)
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const (
	appName    = "SyntaxOrigin Multi Tools"
	appVersion = "1.0.1"
	repoURL    = "https://github.com/SyntaxOrigin"
)

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            appName,
		Width:            1220,
		Height:           800,
		MinWidth:         1040,
		MinHeight:        680,
		MaxWidth:         3000,
		MaxHeight:        2000,
		BackgroundColour: &options.RGBA{R: 14, G: 18, B: 30, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatalf("uygulama hatası: %v", err)
	}
}
