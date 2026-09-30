package shared

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/db"
)

// FormatJSON writes v to stdout as indented JSON.
func FormatJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintArticle prints a single article as a markdown document or JSON.
func PrintArticle(article ArticleView, asJSON bool) error {
	if asJSON {
		return FormatJSON(article)
	}

	// Markdown document format
	fmt.Printf("# Article %s\n\n", article.ID)
	if article.Title != "" {
		fmt.Printf("**%s**\n\n", article.Title)
	}

	fmt.Printf("## Metadata\n\n")
	fmt.Printf("- Entry: %s\n", article.EntryID)
	fmt.Printf("- Added: %s\n\n", article.CreatedAt)
	fmt.Printf("---\n\n%s\n\n", article.Content)
	PrintAssetsSection(article.Assets)

	return nil
}

// PrintAssetsSection prints the asset list as a markdown section.
func PrintAssetsSection(assetList []db.ArticleAsset) {
	if len(assetList) == 0 {
		return
	}

	fmt.Printf("## Assets\n\n")
	for _, asset := range assetList {
		fmt.Printf("- %s (%s, %s)\n", asset.LogicalPath, assets.FormatSize(asset.SizeBytes), asset.ID)
	}
	fmt.Println()
}

// PrintAttachmentsSection prints the attachment list as a markdown section.
func PrintAttachmentsSection(attachmentList []db.EntryAttachment) {
	if len(attachmentList) == 0 {
		return
	}

	fmt.Printf("## Attachments\n\n")
	for _, att := range attachmentList {
		fmt.Printf("- %s (%s, %s, %s)\n", att.Title, att.FileName, assets.FormatSize(att.SizeBytes), att.ID)
	}
	fmt.Println()
}
