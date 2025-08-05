package docset

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Downloader handles downloading and extracting docsets
type Downloader struct {
	cacheDir string
	client   *http.Client
}

// NewDownloader creates a new docset downloader
func NewDownloader() (*Downloader, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".cache", "gismo", "docsets")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Downloader{
		cacheDir: cacheDir,
		client:   &http.Client{},
	}, nil
}

// Download downloads a docset from a URL and returns the path to the extracted .docset
func (d *Downloader) Download(ctx context.Context, url string, progress func(percent int, message string)) (string, error) {
	// Determine docset name from URL
	docsetName := extractDocsetName(url)
	if docsetName == "" {
		return "", fmt.Errorf("could not determine docset name from URL: %s", url)
	}

	// Check if already cached
	docsetPath := filepath.Join(d.cacheDir, docsetName+".docset")
	if _, err := os.Stat(docsetPath); err == nil {
		if progress != nil {
			progress(100, "Using cached docset")
		}
		return docsetPath, nil
	}

	// Download the archive
	if progress != nil {
		progress(0, "Downloading docset")
	}

	archivePath := filepath.Join(d.cacheDir, docsetName+".tgz")
	if err := d.downloadFile(ctx, url, archivePath); err != nil {
		return "", fmt.Errorf("failed to download docset: %w", err)
	}
	defer os.Remove(archivePath)

	// Extract the archive
	if progress != nil {
		progress(50, "Extracting docset")
	}

	if err := d.extractTarGz(archivePath, d.cacheDir); err != nil {
		return "", fmt.Errorf("failed to extract docset: %w", err)
	}

	// Verify the docset was extracted
	if _, err := os.Stat(docsetPath); err != nil {
		// Sometimes the docset is in a subdirectory
		entries, err := os.ReadDir(d.cacheDir)
		if err != nil {
			return "", fmt.Errorf("failed to read cache directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".docset") {
				return filepath.Join(d.cacheDir, entry.Name()), nil
			}
		}

		return "", fmt.Errorf("docset not found after extraction")
	}

	if progress != nil {
		progress(100, "Download complete")
	}

	return docsetPath, nil
}

// downloadFile downloads a file from a URL
func (d *Downloader) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractTarGz extracts a tar.gz file
func (d *Downloader) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize the file path to prevent directory traversal
		target := filepath.Join(destDir, filepath.Clean(header.Name))

		// Ensure the target path is within destDir
		relPath, err := filepath.Rel(destDir, target)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			// Limit file size to prevent decompression bombs (500MB limit)
			limitedReader := io.LimitReader(tr, 500*1024*1024)
			if _, err := io.Copy(outFile, limitedReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// Set file permissions (safely handle mode conversion)
			// #nosec G115 -- header.Mode is from tar which uses int64, we mask to valid permission bits
			mode := os.FileMode(header.Mode & 0777)
			if err := os.Chmod(target, mode); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractDocsetName extracts the docset name from a URL
func extractDocsetName(url string) string {
	// Handle Kapeli feeds URLs like https://kapeli.com/feeds/Go.tgz
	if strings.Contains(url, "kapeli.com/feeds/") {
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			return strings.TrimSuffix(filename, ".tgz")
		}
	}

	// Handle GitHub user contributions
	if strings.Contains(url, "Dash-User-Contributions") {
		// Extract from path like .../docsets/DocsetName/...
		if idx := strings.Index(url, "/docsets/"); idx >= 0 {
			remaining := url[idx+9:]
			if slashIdx := strings.Index(remaining, "/"); slashIdx > 0 {
				return remaining[:slashIdx]
			}
		}
	}

	// Generic handling - get filename without extension
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// Remove common archive extensions
		for _, ext := range []string{".tgz", ".tar.gz", ".zip"} {
			if strings.HasSuffix(filename, ext) {
				return strings.TrimSuffix(filename, ext)
			}
		}
		return filename
	}

	return ""
}
