package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

// Slugify converts a title to a URL-safe slug
func slugify(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	return slug
}

// FrontMatter represents the YAML front matter for exported files
type FrontMatter struct {
	Title    string   `yaml:"title"`
	KbID     string   `yaml:"kb_id"`
	ParentID string   `yaml:"parent_id,omitempty"`
	Aliases  []string `yaml:"aliases,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	Created  string   `yaml:"created"`
	Updated  string   `yaml:"updated,omitempty"`
	KbSource string   `yaml:"kb_source"`
}

// ExistingFile tracks an existing exported file with its kb_id
type ExistingFile struct {
	Path  string
	KbID  string
	IsDir bool
}

// ParseFrontMatter extracts kb_id from YAML front matter
func parseFrontMatter(content []byte) (string, error) {
	// Check for YAML front matter delimiter
	if len(content) < 4 || !bytes.HasPrefix(content, []byte("---")) {
		return "", nil
	}

	// Find the closing ---
	endIdx := bytes.Index(content[3:], []byte("\n---"))
	if endIdx == -1 {
		return "", nil
	}
	endIdx += 3 // Account for the skipped bytes

	frontMatter := content[3 : endIdx+1]

	var fm FrontMatter
	if err := yaml.Unmarshal(frontMatter, &fm); err != nil {
		return "", err
	}

	return fm.KbID, nil
}

// ScanOutputDirectory scans the output directory for existing kb_ids
func scanOutputDirectory(outputDir string) (map[string]*ExistingFile, error) {
	existing := make(map[string]*ExistingFile)

	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		kbID, err := parseFrontMatter(content)
		if err != nil {
			return err
		}

		if kbID != "" {
			relPath, _ := filepath.Rel(outputDir, path)
			existing[kbID] = &ExistingFile{
				Path:  relPath,
				KbID:  kbID,
				IsDir: false,
			}
		}

		return nil
	})

	return existing, err
}

// formatFrontMatter creates YAML front matter string
func formatFrontMatter(fm FrontMatter) (string, error) {
	// Ensure aliases has the title as first entry if not empty
	if fm.Title != "" && len(fm.Aliases) == 0 {
		fm.Aliases = []string{fm.Title}
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}

	return "---\n" + string(data) + "---\n\n", nil
}

// parseTags parses comma-separated or space-separated tags
func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}
	// Split by comma or space
	re := regexp.MustCompile(`[,\s]+`)
	parts := re.Split(tags, -1)
	var result []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}

// formatDate formats a timestamp for front matter
func formatDate(timestamp string) string {
	if timestamp == "" {
		return ""
	}
	// Parse the timestamp and format as YYYY-MM-DD
	t, err := time.Parse("2006-01-02 15:04:05", timestamp)
	if err != nil {
		// Try alternative formats
		t, err = time.Parse("2006-01-02T15:04:05Z", timestamp)
		if err != nil {
			return timestamp[:10] // Fallback to just the date part
		}
	}
	return t.Format("2006-01-02")
}

// exportItem bundles an entry with the article views and attachments loaded
// for an export run.
type exportItem struct {
	entry       *db.Entry
	articles    []articleView
	attachments []db.EntryAttachment
}

// articleFileBase returns the export file base name (without .md) for a
// non-primary article file.
func articleFileBase(article articleView) string {
	if fname := slugify(article.Title); fname != "" {
		return fname
	}
	return "article-" + article.ID
}

// entryExportPaths returns the output file paths (relative to outputDir) that
// ExportEntry writes for an entry: the primary entry file plus one file per
// additional article. Path resolution mirrors ExportEntry so the index always
// references the exact files that were written.
func entryExportPaths(entry *db.Entry, articles []articleView, outputDir string) []string {
	slug := slugify(entry.Title)
	if slug == "" {
		slug = entry.ID
	}
	entryPath := resolveExportEntryPath(outputDir, slug, entry.ID)
	paths := []string{filepath.Join(entryPath, slug+".md")}
	for i := 1; i < len(articles); i++ {
		paths = append(paths, filepath.Join(entryPath, articleFileBase(articles[i])+".md"))
	}
	return paths
}

func resolveExportEntryPath(outputDir, slug, entryID string) string {
	entryPath := filepath.Join(outputDir, slug)
	info, err := os.Stat(entryPath)
	if err != nil {
		return entryPath
	}
	if !info.IsDir() {
		return filepath.Join(outputDir, fmt.Sprintf("%s-%s", slug, entryID))
	}

	mainFile := filepath.Join(entryPath, slug+".md")
	content, err := os.ReadFile(mainFile)
	if err != nil {
		return entryPath
	}
	kbID, err := parseFrontMatter(content)
	if err != nil || kbID == "" || kbID == entryID {
		return entryPath
	}
	return filepath.Join(outputDir, fmt.Sprintf("%s-%s", slug, entryID))
}

// appendAssetLinks creates the article asset section of an exported entry file.
func appendAssetLinks(content string, articleID string, assetList []db.ArticleAsset) string {
	if len(assetList) == 0 {
		return content
	}

	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n\n## Assets\n\n")
	for _, asset := range assetList {
		link := assetstore.AssetLinkPath(articleID, asset.LogicalPath)
		fmt.Fprintf(&b, "- [%s](%s)\n", asset.LogicalPath, link)
	}
	return b.String()
}

// appendAttachmentsSection creates the entry attachments section of an
// exported entry file with relative markdown links.
func appendAttachmentsSection(content string, attachments []db.EntryAttachment) string {
	if len(attachments) == 0 {
		return content
	}

	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n\n## Attachments\n\n")
	for _, att := range attachments {
		link := assetstore.AttachmentLinkPath(att.EntryID, att.ID, att.FileName)
		fmt.Fprintf(&b, "- [%s](%s) (`%s`, %s)\n", att.Title, link, att.FileName, assetstore.FormatSize(att.SizeBytes))
	}
	return b.String()
}

func generateEntryFile(entry *db.Entry, article articleView, attachments []db.EntryAttachment) (string, error) {
	fm := FrontMatter{
		Title:    entry.Title,
		KbID:     entry.ID,
		Tags:     parseTags(entry.Tags),
		Created:  formatDate(entry.CreatedAt),
		Updated:  formatDate(entry.UpdatedAt),
		KbSource: "kb",
	}

	frontMatter, err := formatFrontMatter(fm)
	if err != nil {
		return "", err
	}

	// Entry file content: heading + article content + asset and attachment links
	content := fmt.Sprintf("# %s\n\n%s", entry.Title, article.Content)
	content = appendAssetLinks(content, article.ID, article.Assets)
	return frontMatter + appendAttachmentsSection(content, attachments), nil
}

// generateArticleFile creates an article file content
// cleanMarkdownLine strips common markdown formatting so article content can
// be used as a plain-text description.
func cleanMarkdownLine(s string) string {
	s = strings.TrimSpace(s)
	// Drop leading heading marker or list bullet on the first line.
	s = regexp.MustCompile(`(?m)^[#>*\-\d.]\s+`).ReplaceAllString(s, "")
	// Links become their text.
	s = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(s, "$1")
	// Remove emphasis, inline code, and blockquote markers.
	s = regexp.MustCompile(`(\*\*|__|\*|_|`+"`"+`|>)`).ReplaceAllString(s, "")
	// Collapse whitespace and newlines into single spaces.
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// shortDescription extracts the first paragraph of article content as a plain
// text description, truncated for index readability.
func shortDescription(content string) string {
	content = strings.TrimSpace(content)
	if first := strings.Index(content, "\n\n"); first >= 0 {
		content = content[:first]
	}
	desc := cleanMarkdownLine(content)
	const maxLen = 160
	if len(desc) > maxLen {
		desc = strings.TrimSpace(desc[:maxLen]) + "…"
	}
	return desc
}

// formatTags renders parsed tags as Obsidian tag references, e.g. "#bug #cache".
func formatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	tagged := make([]string, 0, len(tags))
	for _, t := range tags {
		tagged = append(tagged, "#"+t)
	}
	return strings.Join(tagged, " ")
}

// indexWikilink renders an Obsidian wikilink for a file relative to the vault
// root (outputDir). Basenames are used when unique - matching Obsidian's own
// resolution - and vault-relative paths otherwise, so links stay unambiguous
// when two entries export files with the same basename.
func indexWikilink(path, outputDir, display string, basenames map[string]int) string {
	rel, err := filepath.Rel(outputDir, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	target := strings.TrimSuffix(filepath.Base(rel), ".md")
	if basenames[target] > 1 {
		target = strings.TrimSuffix(rel, ".md")
	}
	if display == "" {
		return "[" + target + "]"
	}
	return fmt.Sprintf("[[%s|%s]]", target, display)
}

// indexFile describes one exported file listed in the index.
type indexFile struct {
	path    string // output-absolute path
	heading string
	desc    string
	tags    string
}

// generateIndex builds the Obsidian INDEX.md content listing every exported
// file with a wikilink, its heading, a short description, and the entry tags.
func generateIndex(entries []exportItem, outputDir string, generatedAt time.Time) (string, error) {
	fm := FrontMatter{
		Title:    "Knowledge Base Index",
		Created:  generatedAt.Format("2006-01-02"),
		KbSource: "kb",
	}
	front, err := formatFrontMatter(fm)
	if err != nil {
		return "", err
	}

	// Collect every exported file and its link target before rendering so
	// basename uniqueness is known across the whole vault.
	type entryFiles struct {
		title string
		files []indexFile
	}
	all := make([]entryFiles, 0, len(entries))
	basenames := make(map[string]int)
	for _, e := range entries {
		ef := entryFiles{title: e.entry.Title}

		paths := entryExportPaths(e.entry, e.articles, outputDir)
		tags := formatTags(parseTags(e.entry.Tags))

		for i, p := range paths {
			rel, err := filepath.Rel(outputDir, p)
			if err != nil {
				rel = p
			}
			basename := strings.TrimSuffix(filepath.Base(rel), ".md")
			basenames[basename]++

			f := indexFile{path: p, tags: tags}
			if i == 0 {
				// Primary file: heading is always the entry title.
				f.heading = e.entry.Title
				if len(e.articles) > 0 {
					f.desc = shortDescription(e.articles[0].Content)
				} else {
					f.desc = "*No content*"
				}
			} else {
				f.heading = e.articles[i].Title
				f.desc = shortDescription(e.articles[i].Content)
			}
			ef.files = append(ef.files, f)
		}
		all = append(all, ef)
	}

	var b strings.Builder
	b.WriteString(front)
	b.WriteString("# Knowledge Base Index\n\n")

	noun := "entries"
	if len(entries) == 1 {
		noun = "entry"
	}
	fmt.Fprintf(&b, "Exported %d %s from kb on %s.\n\n", len(entries), noun, fm.Created)

	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].title) < strings.ToLower(all[j].title)
	})

	for _, ef := range all {
		fmt.Fprintf(&b, "## %s\n\n", ef.title)
		for _, f := range ef.files {
			fmt.Fprintf(&b, "- %s - %s", indexWikilink(f.path, outputDir, f.heading, basenames), f.desc)
			if f.tags != "" {
				fmt.Fprintf(&b, " %s", f.tags)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func generateArticleFile(entry *db.Entry, article articleView) (string, error) {
	fm := FrontMatter{
		Title:    article.Title,
		KbID:     article.ID,
		ParentID: entry.ID,
		Tags:     parseTags(entry.Tags),
		Created:  formatDate(article.CreatedAt),
		KbSource: "kb",
	}

	frontMatter, err := formatFrontMatter(fm)
	if err != nil {
		return "", err
	}

	// Article file content: heading + article content
	content := fmt.Sprintf("# %s\n\n%s", article.Title, article.Content)
	return frontMatter + appendAssetLinks(content, article.ID, article.Assets), nil
}

func ExportEntry(entry *db.Entry, articles []articleView, attachments []db.EntryAttachment, outputDir, assetsRoot string, withAttachments, dryRun bool) (string, error) {
	// Attachments are exported only with --with-attachments; without the flag
	// they are excluded from the tree, the links, and the dry-run listing.
	if !withAttachments {
		attachments = nil
	}

	slug := slugify(entry.Title)
	if slug == "" {
		slug = entry.ID
	}
	entryPath := resolveExportEntryPath(outputDir, slug, entry.ID)

	if dryRun {
		fmt.Printf("[DRY-RUN] Would create: %s/\n", entryPath)
		mainFile := filepath.Join(entryPath, slug+".md")
		fmt.Printf("[DRY-RUN]   - %s\n", mainFile)
		for i := 1; i < len(articles); i++ {
			a := articles[i]
			fmt.Printf("[DRY-RUN]   - %s\n", filepath.Join(entryPath, articleFileBase(a)+".md"))
		}
		for _, article := range articles {
			for _, asset := range article.Assets {
				fmt.Printf("[DRY-RUN]   - %s\n", filepath.Join(entryPath, "assets", asset.ArticleID, filepath.FromSlash(asset.LogicalPath)))
			}
		}
		for _, att := range attachments {
			fmt.Printf("[DRY-RUN]   - %s\n", filepath.Join(entryPath, "attachments", att.ID, filepath.FromSlash(att.FileName)))
		}
		return entryPath, nil
	}

	if err := os.MkdirAll(entryPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	var content string
	var err error
	if len(articles) > 0 {
		content, err = generateEntryFile(entry, articles[0], attachments)
	} else {
		fm := FrontMatter{
			Title:    entry.Title,
			KbID:     entry.ID,
			Tags:     parseTags(entry.Tags),
			Created:  formatDate(entry.CreatedAt),
			Updated:  formatDate(entry.UpdatedAt),
			KbSource: "kb",
		}
		content, _ = formatFrontMatter(fm)
		content += fmt.Sprintf("# %s\n\n*No content*", entry.Title)
		content = appendAttachmentsSection(content, attachments)
	}
	if err != nil {
		return "", err
	}
	mainFile := filepath.Join(entryPath, slug+".md")
	if err := os.WriteFile(mainFile, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write entry file: %w", err)
	}

	for i := 1; i < len(articles); i++ {
		article := articles[i]
		content, err := generateArticleFile(entry, article)
		if err != nil {
			return "", err
		}

		articleFile := filepath.Join(entryPath, articleFileBase(article)+".md")
		if err := os.WriteFile(articleFile, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write article file: %w", err)
		}
	}

	for _, article := range articles {
		for _, asset := range article.Assets {
			if err := assetstore.ExportAssetFile(assetsRoot, entryPath, asset); err != nil {
				return "", fmt.Errorf("failed to export asset %s: %w", asset.ID, err)
			}
		}
	}

	for _, att := range attachments {
		if err := assetstore.ExportAttachmentFile(assetsRoot, entryPath, att); err != nil {
			return "", fmt.Errorf("failed to export attachment %s: %w", att.ID, err)
		}
	}

	return entryPath, nil
}

func (c *Commands) export() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export entries to Obsidian-compatible markdown files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "Output directory",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "entry",
				Aliases: []string{"e"},
				Usage:   "Export single entry by ID",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Export all entries",
			},
			&cli.BoolFlag{
				Name:  "with-attachments",
				Usage: "Also export entry attachments into <entry>/attachments/",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Skip overwrite confirmation prompt",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Preview without writing",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()
			if err := database.Init(); err != nil {
				return err
			}

			outputDir := cmd.String("output")
			entryID := cmd.String("entry")
			exportAll := cmd.Bool("all")
			withAttachments := cmd.Bool("with-attachments")
			force := cmd.Bool("force")
			dryRun := cmd.Bool("dry-run")

			// Validate flags
			if entryID == "" && !exportAll {
				return fmt.Errorf("either --entry or --all flag is required")
			}

			// Create output directory if needed
			if !dryRun {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					return fmt.Errorf("failed to create output directory: %w", err)
				}
			}

			// Scan existing files for conflict detection
			existingFiles := make(map[string]*ExistingFile)
			if !dryRun {
				if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
					existingFiles, err = scanOutputDirectory(outputDir)
					if err != nil {
						return fmt.Errorf("failed to scan existing files: %w", err)
					}
				}
			}

			// Collect entries to export
			var entries []exportItem

			if entryID != "" {
				// Single entry
				entry, err := database.GetEntry(entryID)
				if err != nil {
					return fmt.Errorf("entry not found: %w", err)
				}
				articles, err := database.GetArticles(entryID)
				if err != nil {
					return err
				}
				views, err := loadArticleViews(database, articles)
				if err != nil {
					return err
				}
				attachments, err := database.ListEntryAttachments(entryID)
				if err != nil {
					return err
				}
				entries = append(entries, exportItem{entry: entry, articles: views, attachments: attachments})
			} else {
				// All entries
				allEntries, err := database.ListEntries()
				if err != nil {
					return err
				}
				for i := range allEntries {
					e := &allEntries[i]
					articles, err := database.GetArticles(e.ID)
					if err != nil {
						return err
					}
					views, err := loadArticleViews(database, articles)
					if err != nil {
						return err
					}
					attachments, err := database.ListEntryAttachments(e.ID)
					if err != nil {
						return err
					}
					entries = append(entries, exportItem{entry: e, articles: views, attachments: attachments})
				}
			}

			// Track export decisions and the entries actually written
			exportAllPrompt := false
			var exported []exportItem

			for _, e := range entries {
				// Check for conflicts
				if existing, ok := existingFiles[e.entry.ID]; ok && !force && !exportAllPrompt {
					fmt.Printf("Found existing: kb_id %q → %s\n", existing.KbID, existing.Path)

					if !dryRun {
						fmt.Print("[Y]es, [N]o, [A]ll, [Q]uit: ")
						reader := bufio.NewReader(os.Stdin)
						input, _ := reader.ReadString('\n')
						input = strings.TrimSpace(strings.ToUpper(input))

						switch input {
						case "Q":
							fmt.Println("Cancelled")
							return nil
						case "A":
							exportAllPrompt = true
						case "N":
							continue
						}
					}
				}

				// Export the entry
				if dryRun {
					fmt.Printf("[DRY-RUN] Export: %s (%s)\n", e.entry.Title, e.entry.ID)
					if _, err := ExportEntry(e.entry, e.articles, e.attachments, outputDir, cfg.AssetsPath, withAttachments, true); err != nil {
						return err
					}
				} else {
					path, err := ExportEntry(e.entry, e.articles, e.attachments, outputDir, cfg.AssetsPath, withAttachments, false)
					if err != nil {
						return fmt.Errorf("failed to export %s: %w", e.entry.ID, err)
					}
					exported = append(exported, e)
					fmt.Printf("Exported: %s (%s) → %s\n", e.entry.Title, e.entry.ID, path)
				}
			}

			// Knowledge base index: always written, referencing the files that
			// were actually exported in this run.
			indexPath := filepath.Join(outputDir, "INDEX.md")
			indexContent, err := generateIndex(exported, outputDir, time.Now())
			if err != nil {
				return fmt.Errorf("failed to generate index: %w", err)
			}

			if dryRun {
				fmt.Printf("[DRY-RUN] Would write: %s\n", indexPath)
			} else {
				writeIndex := true
				if _, statErr := os.Stat(indexPath); statErr == nil && !force && !exportAllPrompt {
					fmt.Printf("Found existing: %s\n", indexPath)
					fmt.Print("[Y]es, [N]o, [A]ll, [Q]uit: ")
					reader := bufio.NewReader(os.Stdin)
					input, _ := reader.ReadString('\n')
					switch strings.TrimSpace(strings.ToUpper(input)) {
					case "Q":
						fmt.Println("Cancelled")
						return nil
					case "N":
						writeIndex = false
					}
				}

				if writeIndex {
					if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
						return fmt.Errorf("failed to write index: %w", err)
					}
					fmt.Printf("Wrote index: %s\n", indexPath)
				}
			}

			if dryRun {
				fmt.Println("\n[DRY-RUN complete - no files written]")
			}

			return nil
		},
	}
}
