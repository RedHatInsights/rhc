package localization

import (
	"fmt"

	"github.com/snapcore/go-gettext"
)

var (
	domain  *gettext.TextDomain
	catalog gettext.Catalog
)

// Init initializes the translator with the given locale directory.
// It automatically detects the user's locale from environment variables.
// path: path to the directory containing locale directories (e.g., 'locale/')
// Example: Init("locale")
func Init(path string) {
	// Create the text domain
	domain = &gettext.TextDomain{
		Name:      "rhc",
		LocaleDir: path,
	}

	// Get available locales and auto-detect user's preference
	languages := gettext.UserLanguages()
	if len(languages) > 0 {
		catalog = domain.Locale(languages...)
	} else {
		// Fallback to UserLocale if no specific languages found
		catalog = domain.UserLocale()
	}
}

// GetLocale tries to get current locale
func GetLocale() string {
	// Get available locales and auto-detect user's preference
	languages := gettext.UserLanguages()
	if len(languages) > 0 {
		return languages[0]
	}
	return ""
}

// T translates a simple string
func T(msgid string) string {
	return catalog.Gettext(msgid)
}

// TF translates a string with formatting
func TF(msgid string, args ...interface{}) string {
	translated := catalog.Gettext(msgid)
	return fmt.Sprintf(translated, args...)
}

// TN translates with plural forms
func TN(singular, plural string, n int) string {
	translated := catalog.NGettext(singular, plural, uint32(n))
	return fmt.Sprintf(translated, n)
}

// TNF translates with plural forms and formatting
func TNF(singular, plural string, n int, args ...interface{}) string {
	translated := catalog.NGettext(singular, plural, uint32(n))
	return fmt.Sprintf(translated, args...)
}

// TC translates with context
func TC(context, msgid string) string {
	return catalog.PGettext(context, msgid)
}

// TCF translates with context and formatting
func TCF(context, msgid string, args ...interface{}) string {
	translated := catalog.PGettext(context, msgid)
	return fmt.Sprintf(translated, args...)
}
