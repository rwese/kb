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
	defer func() { _ = src.Close() }()

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

// copyFileWithMode streams srcPath to destPath, preserving the regular
// permission bits (special mode bits are stripped) and recording SHA-256 and
// byte size.
func copyFileWithMode(srcPath, destPath string, perm os.FileMode) (string, int64, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = src.Close() }()

	info, err := src.Stat()
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("not a regular file: %s", srcPath)
	}

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
	if err := os.Chmod(destPath, perm); err != nil {
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

// Entry attachment managed-file helpers. Attachments live under
// <assets_root>/entries/<entry-id>/attachments/<attachment-id>/<file-name>,
// a namespace separate from article assets.

// ValidateAttachmentSource rejects symlinks, directories, sockets, devices,
// named pipes, and irregular files. It returns the existing file info for the
// regular file (permission bits survive store and export).
func ValidateAttachmentSource(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlinks are not supported: %s", path)
	}
	isSpecial := info.Mode()&(os.ModeSocket|os.ModeNamedPipe|os.ModeDevice|os.ModeCharDevice|os.ModeIrregular) != 0
	if isSpecial {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	return info, nil
}

// AttachmentFileName extracts a path-safe basename from the source path.
func AttachmentFileName(path string) (string, error) {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == ".." || name == "" || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid file name from path %q", path)
	}
	return name, nil
}

// AttachmentStoreRelPath builds the store-relative path for an attachment.
func AttachmentStoreRelPath(entryID, attachmentID, fileName string) string {
	return filepath.ToSlash(filepath.Join("entries", entryID, "attachments", attachmentID, filepath.FromSlash(fileName)))
}

// StageAttachment validates, copies (preserving regular permission bits, with
// SHA-256 and size), and records the attachment metadata. Callers must remove
// the staged tree with CleanupStagedAttachment on any later database failure.
func StageAttachment(assetsRoot, entryID, attachmentID, sourcePath string) (*db.EntryAttachment, error) {
	info, err := ValidateAttachmentSource(sourcePath)
	if err != nil {
		return nil, err
	}
	fileName, err := AttachmentFileName(sourcePath)
	if err != nil {
		return nil, err
	}

	storeRelPath := AttachmentStoreRelPath(entryID, attachmentID, fileName)
	destPath := filepath.Join(assetsRoot, filepath.FromSlash(storeRelPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		_ = os.RemoveAll(filepath.Dir(destPath))
		return nil, err
	}

	sha, size, err := copyFileWithMode(absPath, destPath, info.Mode().Perm())
	if err != nil {
		_ = os.RemoveAll(filepath.Dir(destPath))
		return nil, err
	}

	return &db.EntryAttachment{
		EntryID:      entryID,
		ID:           attachmentID,
		Title:        "",
		FileName:     fileName,
		OriginalPath: absPath,
		StoreRelPath: storeRelPath,
		SHA256:       sha,
		SizeBytes:    size,
		ModePerm:     int64(info.Mode().Perm()),
	}, nil
}

func CleanupStagedAttachment(assetsRoot, entryID, attachmentID string) {
	_ = os.RemoveAll(filepath.Join(assetsRoot, "entries", entryID, "attachments", attachmentID))
}

// RemoveAttachmentTree removes one attachment's stored bytes.
func RemoveAttachmentTree(assetsRoot, entryID, attachmentID string) error {
	return os.RemoveAll(filepath.Join(assetsRoot, "entries", entryID, "attachments", attachmentID))
}

// RemoveAttachmentFile removes a single stored attachment file and the
// (now empty) attachment directory. Used when a replacement changed the
// stored file name and the fresh bytes live in the same directory.
func RemoveAttachmentFile(assetsRoot, entryID, attachmentID, fileName string) error {
	tree := filepath.Join(assetsRoot, "entries", entryID, "attachments", attachmentID)
	if err := os.Remove(filepath.Join(tree, fileName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(tree) // best-effort: remove dir when empty
	return nil
}

// RemoveEntryAttachmentsTree removes the whole attachment storage tree of an
// entry. Entry deletion removes both the database rows (foreign-key cascade)
// and this tree.
func RemoveEntryAttachmentsTree(assetsRoot, entryID string) error {
	return os.RemoveAll(filepath.Join(assetsRoot, "entries", entryID))
}

// ExportAttachmentFile copies an attachment into the export tree at
// <outputRoot>/assets/attachments/<attachment-id>/<file-name>, preserving
// regular permission bits.
func ExportAttachmentFile(assetsRoot, outputRoot string, att db.EntryAttachment) error {
	srcPath := filepath.Join(assetsRoot, filepath.FromSlash(att.StoreRelPath))
	destPath := filepath.Join(outputRoot, "assets", "attachments", att.ID, filepath.FromSlash(att.FileName))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	_, _, err := copyFileWithMode(srcPath, destPath, os.FileMode(att.ModePerm))
	return err
}

// AttachmentLinkPath is the relative markdown link target for an attachment
// inside an exported entry directory.
func AttachmentLinkPath(entryID, attachmentID, fileName string) string {
	return filepath.ToSlash(filepath.Join("assets", "attachments", attachmentID, filepath.FromSlash(fileName)))
}
