---
name: skillforge
description: Manage AI agent skills with SkillForge CLI. Use when installing, updating, syncing, or removing skills from agentskills.io or custom git repositories. Includes git hook setup for source repositories, and skill registry operations.
---

# SkillForge

CLI tool for managing agent skills across global and local scopes. Skills are copied (not symlinked) to enable proper git versioning.

## Quick Reference

| Command | Description |
|---------|-------------|
| `skillforge install` | Install skills from cached repos |
| `skillforge sync` | Sync installed skills to latest versions |
| `skillforge update [--check]` | Check/update individual skills |
| `skillforge list [--format=...]` | List installed skills |
| `skillforge search <query>` | Search available skills |
| `skillforge remove <name>` | Remove installed skill |
| `skillforge repo add <url>` | Add skill repository |
| `skillforge suggest [--path=<dir>]` | Suggest skills for project |
| `skillforge update-inventory [--dir=<path>]` | Update skill inventory file |

## Efficient Sync

SkillForge optimizes syncing by:

- **Batched fetches**: One `git fetch` per repository, not per skill
- **Content detection**: Only updates skills that actually changed
- **Atomic updates**: Safe file operations

## Inventory Commands

Manage skill inventories in source repositories for efficient change detection during sync.

| Command | Description |
|---------|-------------|
| `skillforge update-inventory` | Scan skills and update the inventory file with current hashes |
| `skillforge update-inventory --dir=<path>` | Update inventory in a specific repository |

The inventory tracks subtree hashes for each skill, enabling sync to detect changes without network calls. Run `update-inventory` before committing skill changes to keep the inventory current.

## Project Structure

```
skillforge/
├── cmd/                     # CLI commands (Cobra)
│   ├── root.go             # Root command + flags
│   ├── sync_cmd.go         # Sync with batching
│   ├── update_cmd.go       # Update command
│   └── repo_cmd.go         # Repo management
├── cmd/internal/
│   ├── repo/               # Git operations
│   ├── config/             # TOML config handling
│   └── search/             # Search index
├── pkg/skill/              # Shared types
└── templates/
    └── pre-commit-hook     # Hook template
```

## Configuration

| Scope | Path | Command Flag |
|-------|------|--------------|
| Global | `~/.config/skillforge/config.toml` | `--global` |
| Local | `.skillforge/config.toml` | `--local` |

## Scopes & Targets

| Scope | Skill Path | Description |
|-------|------------|-------------|
| `pi-global` | `~/.pi/agent/skills/` | Global across all projects |
| `pi-local` | `~/.pi/skills/` | Project-specific |

## Git Hook for Source Repos

Source repositories should include a pre-commit hook to track skill changes. Install the hook:

```bash
# From source repository root
cp templates/pre-commit-hook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

See [git-hook.md](references/git-hook.md) for full setup.

## Common Workflows

### Install from agentskills.io
```bash
skillforge search <skill-name>
skillforge install <skill-name>
```

### Install from git repo
```bash
skillforge repo add <git-url> [--branch=<branch>]
skillforge install <skill-name>
```

### Sync all skills
```bash
skillforge sync [--auto-update|-y]
```

### Update single skill
```bash
skillforge update --check        # Check for updates
skillforge update <skill-name>   # Update specific
```

See [commands.md](references/commands.md) for detailed command docs.
