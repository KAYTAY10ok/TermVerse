package ui

import (
	"fmt"
	"os"
	"strings"
)

// Languages supported by the interface.
var Languages = []string{"auto", "en", "fr"}

var lang = "en"

// SetLanguage selects the interface language ("auto" follows $LANG).
func SetLanguage(l string) {
	if l == "" || l == "auto" {
		l = DetectLanguage()
	}
	if l != "fr" {
		l = "en"
	}
	lang = l
}

// Language returns the active language code.
func Language() string { return lang }

// DetectLanguage guesses the language from the locale environment.
func DetectLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := strings.ToLower(os.Getenv(k)); v != "" {
			if strings.HasPrefix(v, "fr") {
				return "fr"
			}
			return "en"
		}
	}
	return "en"
}

// T translates an English source string into the active language. Unknown
// strings are returned unchanged, so the UI always has readable text.
func T(s string) string {
	if lang == "fr" {
		if v, ok := fr[s]; ok {
			return v
		}
	}
	return s
}

// Tf translates a format string and applies fmt.Sprintf.
func Tf(format string, args ...any) string { return fmt.Sprintf(T(format), args...) }
