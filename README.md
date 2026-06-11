# ayumi

`ayumi` is used for agentic coding--it records user prompts sent to agents and inserts them into Git commit messages.

`ayumi` records only user instructions. It does not store AI responses, transcripts, reasoning, or tool output.

The name `ayumi` comes from the Japanese word 歩み which means the history or steps of something. This tool stores the steps took in agentic coding, hence the naming.

>[!WARNING]
> `ayumi` copies raw prompts into your git commit messages. Do not include secrets, credentials, or anything else you do not want published and recorded to commit messages (you probably shouldn't send them anyways as prompts to coding agents as well).

## Commit Message Format

```text
feat: add JWT middleware

AI Instructions:
> Add JWT authentication

> Move it into middleware

> Validate issuer and audience
```

Prompts are inserted as recorded. Each prompt is inserted as a Markdown quote block, with a blank line between prompts.

## Installation

Install with `go install` like this:

```sh
go install github.com/stefafafan/ayumi@latest
```

### Hook Setup

#### Claude Code / Codex Hooks

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

#### Git Hooks

For Git 2.54 or newer, use Git's [config-based hooks](https://git-scm.com/docs/git-hook/2.54.0).

```ini
[hook "ayumi"]
        event = prepare-commit-msg
        command = sh -c '/absolute/path/to/ayumi inject "$1"' ayumi
```

The `sh -c` wrapper passes only Git's first `prepare-commit-msg` argument to `ayumi inject`. This avoids forwarding extra hook arguments such as `message` from `git commit -m`.

For older Git versions, create a `prepare-commit-msg` hook file like this and make it executable. Set it as a global hook if needed.

```sh
#!/bin/sh

/absolute/path/to/ayumi inject "$1"
```

## Customization

Configuration is read from `~/.config/ayumi/config.toml`

Here are default settings, manually set these values if you want to update:

```toml
storage_dir = "~/.local/share/ayumi"
heading = "AI Instructions"
```

## Commands

### `ayumi add`

Reads from standard input and stores the user prompt for the current Git repository and branch to file.

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

### `ayumi version`

Prints the current `ayumi` version.

```sh
ayumi version
```
