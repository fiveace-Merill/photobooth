package photos

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

var safeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._ -]+`)

func validateContentType(ct string) error {
	if !allowedContentTypes[strings.ToLower(strings.TrimSpace(ct))] {
		return fmt.Errorf("unsupported_type")
	}
	return nil
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "photo"
	}
	if strings.ContainsAny(name, "/\\") {
		name = strings.ReplaceAll(name, "\\", "/")
		parts := strings.Split(name, "/")
		name = parts[len(parts)-1]
	}
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = safeFilenameChars.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "photo"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}
