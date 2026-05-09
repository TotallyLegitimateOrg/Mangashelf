package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
)

const catboxAPI = "https://catbox.moe/user/api.php"

var catboxUserHash atomic.Value

func init() {
	catboxUserHash.Store("")
}

func ConfigureCatboxUserHash(userHash string) {
	catboxUserHash.Store(strings.TrimSpace(userHash))
}

func UploadToCatbox(ctx context.Context, filename string, content []byte) (string, error) {
	userHash, _ := catboxUserHash.Load().(string)
	return uploadToCatbox(ctx, http.DefaultClient, catboxAPI, userHash, filename, content)
}

func uploadToCatbox(ctx context.Context, client *http.Client, apiURL string, userHash string, filename string, content []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("reqtype", "fileupload"); err != nil {
		return "", err
	}
	if userHash = strings.TrimSpace(userHash); userHash != "" {
		if err := writer.WriteField("userhash", userHash); err != nil {
			return "", err
		}
	}

	part, err := writer.CreateFormFile("fileToUpload", filename)
	if err != nil {
		return "", err
	}

	if _, err := part.Write(content); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("catbox upload failed: %s", strings.TrimSpace(string(data)))
	}

	url := strings.TrimSpace(string(data))
	if !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("catbox returned unexpected response for %s: %s", filepath.Base(filename), url)
	}

	return url, nil
}
