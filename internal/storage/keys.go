package storage

import "encoding/hex"

// Key layout is fixed and contains only IDs/kind segments.
// NEVER include user-provided text (filenames) in object keys.
func OriginalKey(photographerID, shootID, photoID [16]byte) string {
	return "p/" + hex.EncodeToString(photographerID[:]) + "/" + hex.EncodeToString(shootID[:]) + "/" + hex.EncodeToString(photoID[:]) + "/original"
}

func PreviewKey(photographerID, shootID, photoID [16]byte) string {
	return "p/" + hex.EncodeToString(photographerID[:]) + "/" + hex.EncodeToString(shootID[:]) + "/" + hex.EncodeToString(photoID[:]) + "/preview.jpg"
}

func ThumbKey(photographerID, shootID, photoID [16]byte) string {
	return "p/" + hex.EncodeToString(photographerID[:]) + "/" + hex.EncodeToString(shootID[:]) + "/" + hex.EncodeToString(photoID[:]) + "/thumb.jpg"
}
