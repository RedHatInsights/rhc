package localization

import (
	"os"
)

// GetLocale tries to get current locale
func GetLocale() string {
	// FIXME: Locale should be detected in more reliable way. We are going to support
	//        localization in better way. Maybe we could use following go module
	//        https://github.com/Xuanwo/go-locale. Maybe some other will be better.
	locale := os.Getenv("LANG")
	return locale
}
