package services

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadToCatboxOmitsUserHashWhenUnset(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields, filename, content := readUploadRequest(t, r)
		if got := fields["reqtype"]; got != "fileupload" {
			t.Fatalf("expected reqtype=fileupload, got %q", got)
		}
		if got := fields["userhash"]; got != "" {
			t.Fatalf("expected empty userhash, got %q", got)
		}
		if filename != "page-001.png" {
			t.Fatalf("expected uploaded filename page-001.png, got %q", filename)
		}
		if content != "image-bytes" {
			t.Fatalf("expected uploaded content, got %q", content)
		}
		_, _ = w.Write([]byte("https://files.catbox.moe/page-001.png"))
	}))
	defer server.Close()

	url, err := uploadToCatbox(context.Background(), server.Client(), server.URL, "", "page-001.png", []byte("image-bytes"))
	if err != nil {
		t.Fatalf("uploadToCatbox returned error: %v", err)
	}
	if url != "https://files.catbox.moe/page-001.png" {
		t.Fatalf("expected uploaded URL, got %q", url)
	}
}

func TestUploadToCatboxIncludesUserHashWhenSet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields, _, _ := readUploadRequest(t, r)
		if got := fields["userhash"]; got != "secret-hash" {
			t.Fatalf("expected userhash to be forwarded, got %q", got)
		}
		_, _ = w.Write([]byte("https://files.catbox.moe/page-001.png"))
	}))
	defer server.Close()

	url, err := uploadToCatbox(context.Background(), server.Client(), server.URL, " secret-hash ", "page-001.png", []byte("image-bytes"))
	if err != nil {
		t.Fatalf("uploadToCatbox returned error: %v", err)
	}
	if url != "https://files.catbox.moe/page-001.png" {
		t.Fatalf("expected uploaded URL, got %q", url)
	}
}

func readUploadRequest(t *testing.T, r *http.Request) (map[string]string, string, string) {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("expected multipart content type, got %q", mediaType)
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	fields := make(map[string]string)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read multipart body: %v", err)
		}
		defer part.Close()

		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		if part.FormName() != "fileToUpload" {
			fields[part.FormName()] = string(data)
			continue
		}
		return fields, part.FileName(), string(data)
	}

	t.Fatal("expected fileToUpload part")
	return nil, "", ""
}
