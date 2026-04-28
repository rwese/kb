package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

// generateEntryFile creates the main entry file content
func generateEntryFile(entry *db.Entry, article *db.Article) (string, error) {
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

	// Entry file content: heading + article content
	content := fmt.Sprintf("# %s\n\n%s", entry.Title, article.Content)
	return frontMatter + content, nil
}

// generateArticleFile creates an article file content
func generateArticleFile(entry *db.Entry, article *db.Article) (string, error) {
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
	return frontMatter + content, nil
}

// ExportEntry exports a single entry to the output directory
// Layout:
//   - Single article: output/entry-title.md
//   - Multiple articles: output/entry-title/article-title.md
func ExportEntry(entry *db.Entry, articles []db.Article, outputDir string, dryRun bool) (string, error) {
	slug := slugify(entry.Title)

	// Determine if we need a folder (multiple articles) or direct file (single article)
	hasMultipleArticles := len(articles) > 1

	var entryPath string
	if hasMultipleArticles {
		// Multiple articles: use folder layout
		entryPath = filepath.Join(outputDir, slug)

		// Check for collision and resolve
		if _, err := os.Stat(entryPath); err == nil {
			entryPath = filepath.Join(outputDir, fmt.Sprintf("%s-%s", slug, entry.ID))
		}
	} else {
		// Single article: direct file at root
		entryPath = filepath.Join(outputDir, slug+".md")

		// Check for collision and resolve
		if _, err := os.Stat(entryPath); err == nil {
			entryPath = filepath.Join(outputDir, fmt.Sprintf("%s-%s.md", slug, entry.ID))
		}
	}

	if dryRun {
		if hasMultipleArticles {
			fmt.Printf("[DRY-RUN] Would create: %s/\n", entryPath)
			for _, a := range articles {
				fname := slugify(a.Title) + ".md"
				if a.Title == "" {
					fname = "article-" + a.ID + ".md"
				}
				fmt.Printf("[DRY-RUN]   - %s\n", filepath.Join(entryPath, fname))
			}
		} else {
			fmt.Printf("[DRY-RUN] Would create: %s\n", entryPath)
		}
		return entryPath, nil
	}

	if hasMultipleArticles {
		// Create directory for multiple articles
		if err := os.MkdirAll(entryPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}

		// Export first article as entry file in the folder
		if len(articles) > 0 {
			content, err := generateEntryFile(entry, &articles[0])
			if err != nil {
				return "", err
			}
			mainFile := filepath.Join(entryPath, slug+".md")
			if err := os.WriteFile(mainFile, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("failed to write entry file: %w", err)
			}

			// Export remaining articles as separate files
			for i := 1; i < len(articles); i++ {
				article := &articles[i]
				content, err := generateArticleFile(entry, article)
				if err != nil {
					return "", err
				}

				fname := slugify(article.Title)
				if fname == "" {
					fname = "article-" + article.ID
				}
				fname += ".md"

				articleFile := filepath.Join(entryPath, fname)
				if err := os.WriteFile(articleFile, []byte(content), 0644); err != nil {
					return "", fmt.Errorf("failed to write article file: %w", err)
				}
			}
		}
	} else {
		// Single article or no articles: direct file
		var content string
		var err error

		if len(articles) > 0 {
			content, err = generateEntryFile(entry, &articles[0])
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
		}

		if err != nil {
			return "", err
		}

		if err := os.WriteFile(entryPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write entry file: %w", err)
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
			defer database.Close()

			outputDir := cmd.String("output")
			entryID := cmd.String("entry")
			exportAll := cmd.Bool("all")
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
			var entries []struct {
				entry    *db.Entry
				articles []db.Article
			}

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
				entries = append(entries, struct {
					entry    *db.Entry
					articles []db.Article
				}{entry: entry, articles: articles})
			} else {
				// All entries
				allEntries, err := database.ListEntries()
				if err != nil {
					return err
				}
				for _, e := range allEntries {
					articles, err := database.GetArticles(e.ID)
					if err != nil {
						return err
					}
					entries = append(entries, struct {
						entry    *db.Entry
						articles []db.Article
					}{entry: &e, articles: articles})
				}
			}

			// Track export decisions
			exportAllPrompt := false

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
				} else {
					path, err := ExportEntry(e.entry, e.articles, outputDir, false)
					if err != nil {
						return fmt.Errorf("failed to export %s: %w", e.entry.ID, err)
					}
					fmt.Printf("Exported: %s (%s) → %s\n", e.entry.Title, e.entry.ID, path)
				}
			}

			if dryRun {
				fmt.Println("\n[DRY-RUN complete - no files written]")
			}

			return nil
		},
	}
}
