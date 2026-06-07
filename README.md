# ayumi

`ayumi` records prompts sent to AI coding agents and inserts them into Git commit messages.

It records only user instructions. It does not store AI responses, transcripts, reasoning, or tool output.

## Flow

```text
User prompt
-> Claude Code / Codex UserPromptSubmit hook
-> ayumi add
-> local prompt log

git commit
-> prepare-commit-msg hook
-> ayumi inject
-> commit message includes AI Instructions
```

## Install

If this repository is available remotely, install with:

```sh
go install github.com/stefafafan/ayumi@latest
```

For local development from this repository, install the current checkout with:

```sh
go install .
```

`go install` writes the `ayumi` binary to `$GOBIN`, or to `$GOPATH/bin` when `GOBIN` is unset. Make sure that directory is on your `PATH`.

Alternatively, build a local binary without installing it:

```sh
go build -o ayumi .
```

## Commands

### `ayumi add`

Reads the hook input from standard input and stores the user prompt for the current Git repository and branch.

```sh
ayumi add
```

### `ayumi inject <commit-message-file>`

Appends prompts recorded after the previous commit time to the commit message file.

```sh
ayumi inject .git/COMMIT_EDITMSG
```

If no prompts were recorded after the previous commit, nothing is inserted.

During rebase, cherry-pick, merge, and revert operations, `ayumi inject` does nothing so Git can reuse the existing commit message.

## Commit Message Format

```text
feat: add JWT middleware

AI Instructions:
- Add JWT authentication
- Move it into middleware
- Validate issuer and audience
```

Prompts are inserted as recorded. Multiline prompts are preserved as continuation lines under the same bullet.

## Storage

Prompt logs are stored outside the repository by default:

```text
~/.local/share/ayumi
```

Logs are separated by repository and branch. The repository identifier is `remote.origin.url`; if a repository has no `origin` remote, the repository root path is used as a local fallback. Identifiers are hashed before they are used in file paths.

## Configuration

Configuration is read from:

```text
~/.config/ayumi/config.toml
```

Supported settings:

```toml
storage_dir = "~/.local/share/ayumi"
heading = "AI Instructions"
```

Changing `heading` changes the inserted commit-message section:

```toml
heading = "Prompt History"
```

```text
feat: add JWT middleware

Prompt History:
- Add JWT authentication
```

## Hook Setup

`ayumi` does not install hooks automatically.

### Claude Code

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ayumi add"
          }
        ]
      }
    ]
  }
}
```

### Codex

Codex hooks are enabled by default. To install the `ayumi` prompt recorder for this repository, create `.codex/hooks.json` at the repository root:

```sh
mkdir -p .codex
cat > .codex/hooks.json <<'EOF'
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ayumi add"
          }
        ]
      }
    ]
  }
}
EOF
```

Use a full path if the `ayumi` binary is not on the `PATH` seen by Codex:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/absolute/path/to/ayumi add"
          }
        ]
      }
    ]
  }
}
```

Start Codex in the repository, then run `/hooks` in the Codex CLI to review and trust the new `UserPromptSubmit` hook. Codex skips non-managed hooks until their exact definitions are trusted, and changed hook definitions need to be reviewed again.

To install the same hook for every repository, put the same JSON in `~/.codex/hooks.json` instead of `<repo>/.codex/hooks.json`.

### Git

#### Per-repository hook

Create `.git/hooks/prepare-commit-msg`:

```sh
#!/bin/sh

ayumi inject "$1"
```

Make it executable:

```sh
chmod +x .git/hooks/prepare-commit-msg
```

#### Global Git hook

To run `ayumi inject` for commits in every repository, configure a global Git hooks directory:

```sh
mkdir -p ~/.config/git/hooks
cat > ~/.config/git/hooks/prepare-commit-msg <<'EOF'
#!/bin/sh

ayumi inject "$1"
EOF
chmod +x ~/.config/git/hooks/prepare-commit-msg
git config --global core.hooksPath ~/.config/git/hooks
```

Use a full path to the binary if `ayumi` is not on the `PATH` seen by Git:

```sh
#!/bin/sh

/absolute/path/to/ayumi inject "$1"
```

`core.hooksPath` replaces Git's default `.git/hooks` lookup. If you already use a global hooks directory, add the `prepare-commit-msg` script there instead of changing `core.hooksPath`.
