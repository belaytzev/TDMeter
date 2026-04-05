package web

import (
	"crypto/sha256"
	"embed"
	"fmt"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed logo.png
var logoPNG []byte

// LogoHash is a short content hash for cache-busting the logo URL.
var LogoHash string

func init() {
	h := sha256.Sum256(logoPNG)
	LogoHash = fmt.Sprintf("%x", h[:4])
}
