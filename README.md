# ayumi

`ayumi` records prompts sent to AI coding agents and inserts them into Git commit messages.

It records only user instructions. It does not store AI responses, transcripts, reasoning, or tool output.

## Privacy Warning

`ayumi` can copy your prompts into Git commit messages. Commit messages may be pushed to public repositories, shared with collaborators, mirrored, indexed, or kept in Git history permanently. Do not include secrets, credentials, private customer data, or anything else you would not want published in prompts that may be recorded by `ayumi`.

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
> Add JWT authentication

> Move it into middleware

> Validate issuer and audience
```

Prompts are inserted as recorded. Each prompt is inserted as a Markdown quote block, with a blank line between prompts.

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
> Add JWT authentication
```

## Hook Setup

### Claude Code / Codex

Setup the hooks for `UserPromptSubmit` hook like this:

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

### Git

Create a `prepare-commit-msg` like this, and make it an executable. Set it as a global hook if needed.

```sh
#!/bin/sh

/absolute/path/to/ayumi inject "$1"
```
