package embed

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ModelFileName is the GGUF model file name
const ModelFileName = "all-MiniLM-L6-v2-Q4_K_M.gguf"

// Asset URLs for pre-built llama.cpp libraries and models
const (
	// GitHub release base URL - update this to your release location
	GitHubReleaseURL = "https://github.com/rwese/kb-assets/releases/download"
	// Current version of assets
	AssetVersion = "v1.0.0"
)

// AssetInfo contains information about downloadable assets
type AssetInfo struct {
	LibraryFile string
	ModelFile   string
	LibraryURL  string
	ModelURL    string
}

// GetAssetInfo returns download URLs for the current platform
func GetAssetInfo(cacheDir string) (*AssetInfo, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Determine library file based on platform
	var libFile string

	switch goos {
	case "darwin":
		libFile = "libllama_go.dylib"
	case "linux":
		libFile = "libllama_go.so"
	case "windows":
		libFile = "llamago.dll"
	default:
		return nil, fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
	}

	// Build asset directory name (normalize for release artifacts)
	assetDir := goos + "-" + goarch
	switch goos {
	case "darwin":
		if goarch == "arm64" {
			assetDir = "darwin-arm64"
		} else {
			assetDir = "darwin-amd64"
		}
	case "linux":
		if goarch == "arm64" {
			assetDir = "linux-arm64"
		} else {
			assetDir = "linux-amd64"
		}
	case "windows":
		assetDir = "windows-amd64"
	}

	baseURL := fmt.Sprintf("%s/%s/%s", GitHubReleaseURL, AssetVersion, assetDir)

	return &AssetInfo{
		LibraryFile: libFile,
		ModelFile:   ModelFileName,
		LibraryURL:  fmt.Sprintf("%s/%s", baseURL, libFile),
		ModelURL:    fmt.Sprintf("%s/%s", baseURL, ModelFileName),
	}, nil
}

// DownloadAsset downloads a file with progress reporting
func DownloadAsset(url, destPath string, progress func(downloaded, total int64)) error {
	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		return nil // File exists, skip download
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total == -1 {
		total = 0 // Unknown size
	}

	// Create temp file
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer f.Close()

	// Download with progress
	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := f.Write(buf[:n])
			if writeErr != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("failed to write: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("read error: %w", err)
		}
	}

	f.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	return nil
}

// DownloadAll downloads library and model assets
func DownloadAll(cacheDir string, progress func(stage string, downloaded, total int64)) error {
	assets, err := GetAssetInfo(cacheDir)
	if err != nil {
		return err
	}

	// Download library
	libraryPath := filepath.Join(cacheDir, assets.LibraryFile)
	if progress != nil {
		progress("Downloading library...", 0, 0)
	}
	if err := DownloadAsset(assets.LibraryURL, libraryPath, func(d, t int64) {
		if progress != nil && t > 0 {
			progress("Downloading library...", d, t)
		}
	}); err != nil {
		return fmt.Errorf("failed to download library: %w", err)
	}

	// Make library executable on Unix
	if runtime.GOOS != "windows" {
		os.Chmod(libraryPath, 0755)
	}

	// Download model
	modelPath := filepath.Join(cacheDir, assets.ModelFile)
	if progress != nil {
		progress("Downloading model...", 0, 0)
	}
	if err := DownloadAsset(assets.ModelURL, modelPath, func(d, t int64) {
		if progress != nil && t > 0 {
			progress("Downloading model...", d, t)
		}
	}); err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	return nil
}

// CheckAssets verifies that required assets exist
func CheckAssets(cacheDir string) (bool, string) {
	assets, err := GetAssetInfo(cacheDir)
	if err != nil {
		return false, err.Error()
	}

	libraryPath := filepath.Join(cacheDir, assets.LibraryFile)
	modelPath := filepath.Join(cacheDir, assets.ModelFile)

	if _, err := os.Stat(libraryPath); os.IsNotExist(err) {
		return false, fmt.Sprintf("library not found: %s", libraryPath)
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false, fmt.Sprintf("model not found: %s", modelPath)
	}

	return true, ""
}

// GetAssetPaths returns the paths to library and model files
func GetAssetPaths(cacheDir string) (libraryPath, modelPath string, err error) {
	assets, err := GetAssetInfo(cacheDir)
	if err != nil {
		return "", "", err
	}

	libraryPath = filepath.Join(cacheDir, assets.LibraryFile)
	modelPath = filepath.Join(cacheDir, assets.ModelFile)

	return libraryPath, modelPath, nil
}

// LibraryFileFromCache returns the library file path
func LibraryFileFromCache(cacheDir string) string {
	assets, err := GetAssetInfo(cacheDir)
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, assets.LibraryFile)
}

// ModelFileFromCache returns the model file path
func ModelFileFromCache(cacheDir string) string {
	return filepath.Join(cacheDir, ModelFileName)
}

// ParseSize converts a byte count string to int64
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	var multiplier int64 = 1
	if strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	}

	var size int64
	for _, c := range s {
		if c < '0' || c > '9' {
			continue
		}
		size = size*10 + int64(c-'0')
	}

	return size * multiplier, nil
}
