# Git Hook Setup

Source repositories should include a pre-commit hook so SkillForge can efficiently detect which skills changed.

## Pre-commit Hook

The hook tracks skill changes automatically.

### Installation

```bash
# From repository root
cp templates/pre-commit-hook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Or symlink for easy updates:

```bash
ln -s ../../templates/pre-commit-hook .git/hooks/pre-commit
```

### Hook Template Location

In skillforge: `templates/pre-commit-hook`

### What the Hook Does

1. Checks if `skillforge` is in PATH
2. Verifies skills directory exists
3. Tracks skill changes for efficient syncing
4. Reports status (does NOT fail commits)

### Hook Content

```bash
#!/bin/bash
# SkillForge Skill Tracking Hook

set -e

if ! command -v skillforge &> /dev/null; then
    echo "skillforge not found. Install from https://github.com/rwese/skillforge"
    exit 0
fi

REPO_ROOT=$(git rev-parse --show-toplevel)

if [ ! -d "$REPO_ROOT/skills" ] && ! ls "$REPO_ROOT"/*/SKILL.md &> /dev/null; then
    exit 0
fi

echo "Updating skill tracking..."
skillforge update-inventory --dir="$REPO_ROOT" || true

exit 0
```

## CI/CD Integration

For CI, add a step to commit skill changes:

```yaml
# .github/workflows/skills.yml
name: Skill Updates

on:
  push:
    branches: [main]
  pull_request:

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Install skillforge
        run: go install

      - name: Track skill changes
        run: skillforge update-inventory

      - name: Commit changes
        run: |
          git diff --quiet || git commit -m "chore: update skill tracking"
          git push
```

## Manual Usage

If hook isn't installed, update manually:

```bash
skillforge update-inventory [--dir=<path>]
git add <changed-files>
git commit -m "chore: update skill tracking"
```

## Testing the Hook

```bash
# Verify hook exists
ls -la .git/hooks/pre-commit

# Test hook execution
git commit --allow-empty -m "test" --no-verify
```

## Requirements

- `skillforge` binary in PATH
- Git repository with skills
- Skills in root or `skills/` subdirectory
