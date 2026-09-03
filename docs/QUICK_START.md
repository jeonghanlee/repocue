# RepoCue Quick Start

## Scope

This guide installs RepoCue from a source checkout, initializes repository
context, produces the first overview cue, and installs the optional agent
skill.

**Out of scope:** system-wide installation, packaged releases, MCP, and hosted
agent configuration.

## Prerequisites

Source installation requires GNU Make, Go 1.24 or newer, and the GNU
coreutils `install` and `realpath` commands. Confirm the required tools before
starting:

```bash
command -v make
command -v install
make check-tools
```

See [INSTALLATION.md](INSTALLATION.md) for the complete prerequisite list.

## Install RepoCue

From the RepoCue repository root, preview, apply, and verify the default user
installation:

```bash
make install.dry-run
make install.apply
export PATH="$HOME/.local/bin:$PATH"
make install.check
```

The executable is installed as `$HOME/.local/bin/repocue`. See
[INSTALLATION.md](INSTALLATION.md) for custom locations, cross-builds, and
removal.

## Install the Optional Agent Skill

While still in the RepoCue checkout, install the portable
`skills/repocue/SKILL.md` file for Codex:

```bash
REPOCUE_CODEX_SKILL_DIR="${CODEX_HOME:-$HOME/.codex}/skills/repocue"
install -d "$REPOCUE_CODEX_SKILL_DIR"
install -m 0644 skills/repocue/SKILL.md "$REPOCUE_CODEX_SKILL_DIR/SKILL.md"
```

Install the same skill for Claude Code:

```bash
REPOCUE_CLAUDE_SKILL_DIR="$HOME/.claude/skills/repocue"
install -d "$REPOCUE_CLAUDE_SKILL_DIR"
install -m 0644 skills/repocue/SKILL.md "$REPOCUE_CLAUDE_SKILL_DIR/SKILL.md"
```

Start a new agent session after the first installation so the runtime can
discover the skill. The skill remains a standalone repository artifact; it is
not a plugin and does not install RepoCue itself.

## Create the First Cue

Change to the Git repository that RepoCue should describe:

```bash
cd /path/to/git-repository
repocue init .
repocue status
repocue cue --view overview --max-tokens 500
```

The final command prints a structured JSON overview. `--max-tokens 500` limits
the estimated size of the cue output; it does not set the AI model's complete
context limit.

## Ask an Agent

The CLI command may be entered directly, or an agent with the RepoCue skill may
receive a natural-language request:

> Use RepoCue to inspect the current repository before reading files. Start
> with an overview limited to about 500 estimated tokens, and inspect files
> directly only when the cue is incomplete or uncertain.

The agent should run `repocue refresh`, initialize the repository first when
refresh reports that RepoCue is not initialized, and then run:

```bash
repocue cue --view overview --max-tokens 500
```
