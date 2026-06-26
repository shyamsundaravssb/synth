package embed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type ModelFile struct {
	Name string
	URL  string
}

func ModelFileList() []ModelFile {
	const hfBaseURL = "https://huggingface.co/" +
		"sentence-transformers/all-MiniLM-L6-v2/" +
		"resolve/main"
	return []ModelFile{
		{Name: "model.onnx", URL: hfBaseURL + "/onnx/model.onnx"},
		{Name: "tokenizer.json", URL: hfBaseURL + "/tokenizer.json"},
		{Name: "config.json", URL: hfBaseURL + "/config.json"},
		{Name: "tokenizer_config.json", URL: hfBaseURL + "/tokenizer_config.json"},
		{Name: "special_tokens_map.json", URL: hfBaseURL + "/special_tokens_map.json"},
	}
}

type ProgressFunc func(fileName string, downloaded, total int64)

func IsModelComplete(dir string) bool {
	for _, file := range ModelFileList() {
		path := filepath.Join(dir, file.Name)
		stat, err := os.Stat(path)
		if err != nil || stat.Size() == 0 {
			return false
		}
	}
	return true
}

type countingReader struct {
	reader   io.Reader
	fileName string
	total    int64
	progress ProgressFunc
	read     int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	if n > 0 {
		cr.read += int64(n)
		if cr.progress != nil {
			total := cr.total
			if total < 0 {
				total = 0
			}
			cr.progress(cr.fileName, cr.read, total)
		}
	}
	return n, err
}

func downloadFile(ctx context.Context, url, destPath string, progress ProgressFunc, fileName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", fileName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: server returned status %d", fileName, resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	cr := &countingReader{
		reader:   resp.Body,
		fileName: fileName,
		total:    resp.ContentLength,
		progress: progress,
	}

	_, copyErr := io.Copy(f, cr)
	_ = f.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return err
	}
	return nil
}

func DownloadModel(ctx context.Context, destDir string, force bool, progress ProgressFunc) error {
	return downloadFiles(ctx, ModelFileList(), destDir, force, progress)
}

func downloadFiles(ctx context.Context, files []ModelFile, destDir string, force bool, progress ProgressFunc) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	for _, file := range files {
		destPath := filepath.Join(destDir, file.Name)
		if !force {
			stat, err := os.Stat(destPath)
			if err == nil && stat.Size() > 0 {
				continue
			}
		}

		if err := downloadFile(ctx, file.URL, destPath, progress, file.Name); err != nil {
			return fmt.Errorf("model download failed on %s: %w", file.Name, err)
		}
	}
	return nil
}
