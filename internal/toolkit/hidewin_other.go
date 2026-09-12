//go:build !windows

package toolkit

import "os/exec"

// HideConsoleWindow diğer platformlarda yapılacak bir iş yoktur.
func HideConsoleWindow(*exec.Cmd) {}
