//go:build !windows

package main

// windowsLocale diğer platformlarda Windows'a özgü API olmadığı için boş döner.
func windowsLocale() string {
	return ""
}
