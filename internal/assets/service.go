package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/id"
)

type ImportFile struct {
	SourcePath  string
	LogicalPath string
}

func ExpandPaths(paths []string) ([]ImportFile, error) {
	var files []ImportFile
	seen := make(map[string]string)

	for _, input := range paths {
		cleanPath := filepath.Clean(input)
		info, err := os.Lstat(cleanPath)
		if err != nil {
			return nil, err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlinks are not supported: %s", cleanPath)
		}

		if info.IsDir() {
			err := filepath.WalkDir(cleanPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if path == cleanPath {
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("symlinks are not supported: %s", path)
				}
				if d.IsDir() {
					return nil
				}

				rel, err := filepath.Rel(cleanPath, path)
				if err != nil {
					return err
				}
				logicalPath := filepath.ToSlash(rel)
				if prev, ok := seen[logicalPath]; ok {
					return fmt.Errorf("duplicate logical path %q from %s and %s", logicalPath, prev, path)
				}

				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				seen[logicalPath] = absPath
				files = append(files, ImportFile{SourcePath: absPath, LogicalPath: logicalPath})
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}

		logicalPath := filepath.Base(cleanPath)
		if prev, ok := seen[logicalPath]; ok {
			return nil, fmt.Errorf("duplicate logical path %q from %s and %s", logicalPath, prev, cleanPath)
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return nil, err
		}
		seen[logicalPath] = absPath
		files = append(files, ImportFile{SourcePath: absPath, LogicalPath: filepath.ToSlash(logicalPath)})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].LogicalPath < files[j].LogicalPath
	})
	return files, nil
}

func StageImports(assetsRoot, articleID string, files []ImportFile) ([]db.ArticleAsset, error) {
	staged := make([]db.ArticleAsset, 0, len(files))

	for _, file := range files {
		assetID := id.Entry()
		storeRelPath := filepath.ToSlash(filepath.Join(articleID, assetID, filepath.FromSlash(file.LogicalPath)))
		destPath := filepath.Join(assetsRoot, filepath.FromSlash(storeRelPath))
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			CleanupStaged(assetsRoot, staged)
			return nil, err
		}

		sha, size, err := copyWithHash(file.SourcePath, destPath)
		if err != nil {
			CleanupStaged(assetsRoot, staged)
			return nil, err
		}

		staged = append(staged, db.ArticleAsset{
			ID:           assetID,
			ArticleID:    articleID,
			LogicalPath:  file.LogicalPath,
			OriginalPath: file.SourcePath,
			StoreRelPath: storeRelPath,
			SHA256:       sha,
			SizeBytes:    size,
		})
	}

	return staged, nil
}

func CleanupStaged(assetsRoot string, assets []db.ArticleAsset) {
	for _, asset := range assets {
		_ = RemoveAssetTree(assetsRoot, asset)
	}
}

func RemoveAssetTree(assetsRoot string, asset db.ArticleAsset) error {
	return os.RemoveAll(filepath.Join(assetsRoot, asset.ArticleID, asset.ID))
}

func RemoveArticleTree(assetsRoot, articleID string) error {
	return os.RemoveAll(filepath.Join(assetsRoot, articleID))
}

func ExportAssetFile(assetsRoot, outputRoot string, asset db.ArticleAsset) error {
	srcPath := filepath.Join(assetsRoot, filepath.FromSlash(asset.StoreRelPath))
	destPath := filepath.Join(outputRoot, "assets", asset.ArticleID, filepath.FromSlash(asset.LogicalPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	_, _, err := copyWithHash(srcPath, destPath)
	return err
}

func AssetLinkPath(articleID, logicalPath string) string {
	return filepath.ToSlash(filepath.Join("assets", articleID, filepath.FromSlash(logicalPath)))
}

func copyWithHash(srcPath, destPath string) (string, int64, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", 0, err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return "", 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", 0, fmt.Errorf("symlinks are not supported: %s", srcPath)
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_ = dst.Close()
	}()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(dst, hash), src)
	if err != nil {
		_ = os.Remove(destPath)
		return "", 0, err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(destPath)
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

func FormatSize(size int64) string {
	switch {
	case size >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	case size >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func HasPathTraversal(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == ".." || strings.HasPrefix(clean, "../")
}
