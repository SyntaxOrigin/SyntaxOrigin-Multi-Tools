// Backend arabirim metinleri (MSP dosya/klasör diyalog başlıkları) için çeviri
// tablosu. İş mesajları için ön yüzdeki I18N sözlüğü kullanılır (backend bu
// mesajları "@anahtar" biçiminde iletir).
package main

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// backendStrings anahtar -> dil kodu -> metin.
var backendStrings = map[string]map[string]string{
	"dlg.pickInput": {
		"en": "Select the file to convert", "zh": "选择要转换的文件", "hi": "कन्वर्ट करने वाली फ़ाइल चुनें",
		"es": "Selecciona el archivo a convertir", "ar": "اختر الملف المراد تحويله", "fr": "Sélectionnez le fichier à convertir",
		"ms": "Pilih fail untuk ditukar", "pt": "Selecione o arquivo para converter", "ru": "Выберите файл для конвертации",
		"ur": "کنورٹ کرنے کے لیے فائل منتخب کریں", "ja": "変換するファイルを選択", "de": "Zu konvertierende Datei auswählen",
		"tr": "Dönüştürülecek dosyayı seçin", "ko": "변환할 파일 선택", "vi": "Chọn tệp để chuyển đổi",
		"it": "Seleziona il file da convertire", "fa": "فایل برای تبدیل را انتخاب کنید", "pl": "Wybierz plik do konwersji",
		"nl": "Bestand selecteren om te converteren", "uk": "Виберіть файл для конвертації",
	},
	"dlg.pickOutput": {
		"en": "Select the output location", "zh": "选择输出位置", "hi": "आउटपुट स्थान चुनें",
		"es": "Selecciona la ubicación de salida", "ar": "اختر موقع الإخراج", "fr": "Sélectionnez l'emplacement de sortie",
		"ms": "Pilih lokasi output", "pt": "Selecione o local de saída", "ru": "Выберите место сохранения",
		"ur": "آؤٹ پٹ مقام منتخب کریں", "ja": "出力先を選択", "de": "Ausgabeort auswählen",
		"tr": "Çıktı konumunu seçin", "ko": "출력 위치 선택", "vi": "Chọn vị trí xuất",
		"it": "Seleziona la posizione di output", "fa": "محل خروجی را انتخاب کنید", "pl": "Wybierz lokalizację wyjścia",
		"nl": "Uitvoerlocatie selecteren", "uk": "Виберіть розташування виводу",
	},
	"dlg.filterSupported": {
		"en": "Supported files", "zh": "支持的文件", "hi": "समर्थित फ़ाइलें",
		"es": "Archivos compatibles", "ar": "الملفات المدعومة", "fr": "Fichiers pris en charge",
		"ms": "Fail disokong", "pt": "Arquivos compatíveis", "ru": "Поддерживаемые файлы",
		"ur": "معاون فائلیں", "ja": "対応ファイル", "de": "Unterstützte Dateien",
		"tr": "Desteklenen dosyalar", "ko": "지원 파일", "vi": "Tệp được hỗ trợ",
		"it": "File supportati", "fa": "فایل‌های پشتیبانی‌شده", "pl": "Obsługiwane pliki",
		"nl": "Ondersteunde bestanden", "uk": "Підтримувані файли",
	},
	"dlg.filterAll": {
		"en": "All files", "zh": "所有文件", "hi": "सभी फ़ाइलें",
		"es": "Todos los archivos", "ar": "جميع الملفات", "fr": "Tous les fichiers",
		"ms": "Semua fail", "pt": "Todos os arquivos", "ru": "Все файлы",
		"ur": "تمام فائلیں", "ja": "すべてのファイル", "de": "Alle Dateien",
		"tr": "Tüm dosyalar", "ko": "모든 파일", "vi": "Tất cả tệp",
		"it": "Tutti i file", "fa": "همه فایلها", "pl": "Wszystkie pliki",
		"nl": "Alle bestanden", "uk": "Усі файли",
	},
}

func (a *App) tr(key string) string {
	if m, ok := backendStrings[key]; ok {
		if s, ok := m[a.lang]; ok && s != "" {
			return s
		}
		if s, ok := m["tr"]; ok && s != "" {
			return s
		}
	}
	return key
}

// SetLanguage arayüz dilini belirler (MSP diyalog başlıkları için).
func (a *App) SetLanguage(code string) {
	if code != "" {
		a.lang = code
	}
}

// desteklenen dil kodları (ön yüzdeki LANGS listesiyle eşleşir).
var supportedLang = map[string]bool{
	"en": true, "zh": true, "hi": true, "es": true, "ar": true, "fr": true, "ms": true,
	"pt": true, "ru": true, "ur": true, "ja": true, "de": true, "tr": true, "ko": true,
	"vi": true, "it": true, "fa": true, "pl": true, "nl": true, "uk": true,
}

// SystemLanguage işletim sisteminin arayüz dilini ISO 639-1 koduyla döndürür;
// desteklenmeyen kodlar için boş döner (ön yüz İngilizce'ye düşer).
func (a *App) SystemLanguage() string {
	code := systemLocale()
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	switch {
	case code == "id":
		code = "ms"
	case strings.HasPrefix(code, "zh"):
		code = "zh"
	case strings.HasPrefix(code, "pt"):
		code = "pt"
	case strings.HasPrefix(code, "ms"):
		code = "ms"
	}
	if supportedLang[code] {
		return code
	}
	return ""
}

// systemLocale ham sistem dili kodunu döndürür ("tr-TR", "en" vb.).
func systemLocale() string {
	if runtime.GOOS == "windows" {
		return windowsLocale()
	}
	if l := os.Getenv("LANG"); l != "" {
		base := strings.TrimSpace(strings.SplitN(l, "_", 2)[0])
		if base == "" || base == "C" || base == "POSIX" ||
			strings.EqualFold(base, "C.UTF-8") || strings.EqualFold(base, "C.utf8") {
			return ""
		}
		return base
	}
	return ""
}

// windowsLocale Windows kullanıcı arayüz dilini GetUserDefaultLocaleName ile alır.
func windowsLocale() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getLocale := k32.NewProc("GetUserDefaultLocaleName")
	var buf [85]uint16
	r, _, _ := getLocale.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}
