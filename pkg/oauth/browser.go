package oauth

import (
	"github.com/pkg/browser"
)

// OpenBrowser opens the user's default browser to the specified URL
func OpenBrowser(url string) error {
	return browser.OpenURL(url)
}
