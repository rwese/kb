# Commands Reference

## install

Install skills from cached repositories.

```bash
skillforge install [--global|-g] [--local|-l] [skill-name...]
```

**Options:**
- `--global, -g`: Install to global scope
- `--local, -l`: Install to local scope

**Examples:**
```bash
skillforge install github docker          # Install multiple
skillforge install --global github       # Global scope
skillforge install --local tmux         # Local scope
```

## sync

Sync installed skills to latest versions.

```bash
skillforge sync [--auto-update|-y]
```

**Options:**
- `--auto-update, -y`: Update without confirmation

**Behavior:**
- Groups skills by source repository
- Performs ONE fetch per unique repository
- Only updates skills that changed
- Skips dirty (locally modified) skills
- Prompts for migration if needed

**Example output:**
```
Syncing skills in local scope...

Found 2 skill(s) with updates available:

  ↻ github
    content changed
    installed in: pi-local

  ↻ docker
    content changed
    installed in: pi-global, pi-local

Found 1 skill(s) with unchanged content (skipping):
  ⊘ tmux (no changes)

The following skills will be updated:
  - github
  - docker

Proceed with update? [y/N] y

  ✓ github updated
  ✓ docker updated

Sync complete: 2 updated, 0 failed, 0 skipped (dirty), 1 skipped (unchanged)
```

## update

Check for or apply updates to individual skills.

```bash
skillforge update [--check] [skill-name]
```

**Options:**
- `--check`: Only check for updates, don't install

**Examples:**
```bash
skillforge update --check              # Check all
skillforge update --check github     # Check specific
skillforge update github             # Update specific
```

## update-inventory

Track skill changes in source repositories.

```bash
skillforge update-inventory [--dir=<path>]
```

**Options:**
- `--dir`: Path to repository (default: current directory)

**Output:**
```
Scanning skills in /path/to/repo...
Found 5 skills: github, docker, tmux, git, npm

Computing content hashes...
  ✓ github: a1b2c3d4e5f6...
  ✓ docker: b2c3d4e5f6a1...
  ...

Skill tracking updated.
```

## list

List installed skills.

```bash
skillforge list [--format=text|json|compact|detailed] [--repo=<pattern>]
```

**Formats:**
- `text`: Clean output (default)
- `compact`: Tab-separated with source
- `detailed`: Full details with source
- `json`: JSON array

**Examples:**
```bash
skillforge list                         # Default format
skillforge list --format=json          # JSON output
skillforge list --repo=github.com/rwese # Filter by repo
```

## search

Search available skills.

```bash
skillforge search <query> [--json] [--verbose] [--limit=N] [--rebuild]
```

**Options:**
- `--json`: JSON output
- `--verbose`: Include descriptions
- `--limit`: Max results (default: 10)
- `--rebuild`: Rebuild search index first

**Examples:**
```bash
skillforge search docker                  # Search by name
skillforge search github --verbose       # With descriptions
skillforge search api --limit=5          # Limited results
```

## remove

Remove installed skill.

```bash
skillforge remove <name> [--sync] [-f]
```

**Options:**
- `--sync`: Also remove from all registered agents
- `-f, --force`: Force removal

**Examples:**
```bash
skillforge remove github                # Remove locally
skillforge remove --sync github         # Remove everywhere
```

## repo

Manage skill repositories.

### repo add

Add repository to cache.

```bash
skillforge repo add <repo-url> [--branch=<branch>] [--local|-l]
```

**Options:**
- `--branch`: Git branch (default: main)
- `--local`: Add to local scope

### repo list

List cached repositories.

```bash
skillforge repo list [--include-global] [--format=text|json]
```

### repo inspect

Show skills in repository.

```bash
skillforge repo inspect <repo> [--installed]
```

**Options:**
- `--installed`: Only show installed skills

### repo remove

Remove cached repository.

```bash
skillforge repo remove <repo> [--force]
```

### repo update

Update cached repositories.

```bash
skillforge repo update [repo] [--check]
```

**Options:**
- `--check`: Check for updates without pulling

## suggest

Analyze project and suggest skills.

```bash
skillforge suggest [--path=<dir>] [--json] [-i|--wizard]
```

**Options:**
- `--path`: Project directory (default: current)
- `--json`: JSON output
- `-i, --wizard`: Interactive mode

## doctor

Diagnose skillforge issues.

```bash
skillforge doctor [--grimoire] [--clean] [--fix]
```

**Options:**
- `--grimoire`: Show grimoire versions
- `--clean`: Clean cache
- `--fix`: Attempt auto-fix

## config

View/edit configuration.

```bash
skillforge config [--set key=value] [--get key] [--global|-g] [--local|-l]
```

**Examples:**
```bash
skillforge config --get cache_path      # View value
skillforge config --set strict_mode=true -l  # Set local
```
