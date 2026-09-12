//go:build windows

package toolkit

import (
	"os/exec"
	"syscall"
)

// createNoWindow Windows'un CREATE_NO_WINDOW (0x08000000) bayrağıdır;
// konsol alt proseslerin yeni pencere açmasını engeller.
const createNoWindow = 0x08000000

// HideConsoleWindow Windows'ta alt prosesin yeni konsol penceresi açmasını
// önler. yt-dlp, ffmpeg, tar vb. CLI araçları böylece arka planda sessizce
// çalışır. Diğer platformlarda no-op'tur.
func HideConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
