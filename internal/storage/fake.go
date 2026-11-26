package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fake is an in-memory ObjectStore for tests. It is not safe as a production store.
type Fake struct {
	mu        sync.Mutex
	objects   map[string][]byte
	uploads   map[string]*fakeUpload
	getURLs   map[string]string
	putTypes  map[string]string
	deleteLog []string
}

type fakeUpload struct {
	key         string
	contentType string
	parts       map[int32][]byte
}

func NewFake() *Fake {
	return &Fake{objects: map[string][]byte{}, uploads: map[string]*fakeUpload{}, getURLs: map[string]string{}, putTypes: map[string]string{}}
}

func (f *Fake) InitiateMultipart(_ context.Context, key, contentType string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("up_%d_%d", time.Now().UnixNano(), len(f.uploads)+1)
	f.uploads[id] = &fakeUpload{key: key, contentType: contentType, parts: map[int32][]byte{}}
	return id, nil
}

func (f *Fake) PresignUploadPart(_ context.Context, key, uploadID string, partNumber int32, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[uploadID]
	if !ok || u.key != key {
		return "", fmt.Errorf("upload not found")
	}
	if partNumber <= 0 {
		return "", fmt.Errorf("invalid part")
	}
	return fmt.Sprintf("https://fake-upload.local/%s/%s/%d", uploadID, key, partNumber), nil
}

func (f *Fake) AddUploadPart(uploadID string, partNumber int32, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[uploadID]
	if !ok {
		return fmt.Errorf("upload not found")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	u.parts[partNumber] = cp
	return nil
}

func (f *Fake) ListParts(_ context.Context, key, uploadID string) ([]UploadedPart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[uploadID]
	if !ok || u.key != key {
		return nil, fmt.Errorf("upload not found")
	}
	out := make([]UploadedPart, 0, len(u.parts))
	for pn := range u.parts {
		out = append(out, UploadedPart{PartNumber: pn, ETag: fmt.Sprintf("etag-%d", pn)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PartNumber < out[j].PartNumber })
	return out, nil
}

func (f *Fake) CompleteMultipart(_ context.Context, key, uploadID string, parts []UploadedPart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[uploadID]
	if !ok || u.key != key {
		return fmt.Errorf("upload not found")
	}
	var buf bytes.Buffer
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	for _, p := range parts {
		b, ok := u.parts[p.PartNumber]
		if !ok {
			return fmt.Errorf("missing part %d", p.PartNumber)
		}
		buf.Write(b)
	}
	f.objects[key] = buf.Bytes()
	f.putTypes[key] = u.contentType
	delete(f.uploads, uploadID)
	return nil
}

func (f *Fake) AbortMultipart(_ context.Context, key, uploadID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[uploadID]
	if !ok || u.key != key {
		return fmt.Errorf("upload not found")
	}
	delete(f.uploads, uploadID)
	return nil
}

func (f *Fake) Head(_ context.Context, key string) (ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return ObjectInfo{}, fmt.Errorf("not found")
	}
	return ObjectInfo{Size: int64(len(b))}, nil
}

func (f *Fake) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (f *Fake) Put(_ context.Context, key, contentType string, body io.Reader, _ int64) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	f.objects[key] = cp
	f.putTypes[key] = contentType
	return nil
}

func (f *Fake) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteLog = append(f.deleteLog, key)
	delete(f.objects, key)
	return nil
}

func (f *Fake) PresignGet(_ context.Context, key string, _ time.Duration, disposition string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objects[key]; !ok {
		return "", fmt.Errorf("not found")
	}
	u := "https://fake-get.local/" + key
	if disposition != "" {
		u += "?disposition=" + strings.ReplaceAll(disposition, " ", "+")
	}
	f.getURLs[key] = u
	return u, nil
}

func (f *Fake) Object(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, true
}
