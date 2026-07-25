# unclean-git-repos

Scan the immediate subdirectories (one level deep) of a folder and report which
git repositories have **uncommitted changes** (a dirty working tree). Non-git
folders are detected and skipped.

## Build

```sh
make build          # compiles to ./bin/unclean-git-repos
```

Or without make: `go build -o unclean-git-repos .`

## Make targets

Run `make help` to list them:

| Target        | Description                                             |
| ------------- | ------------------------------------------------------- |
| `make build`  | Compile the binary into `./bin`                         |
| `make run`    | Build and run (pass args with `ARGS=...`)               |
| `make install`| Install to `$PREFIX/bin` (default `~/.local/bin`)       |
| `make uninstall` | Remove the installed binary                          |
| `make test`   | Run the test suite                                      |
| `make vet`    | Run `go vet`                                            |
| `make fmt`    | Format sources                                          |
| `make tidy`   | Sync `go.mod`/`go.sum`                                  |
| `make clean`  | Remove build artifacts                                  |

Override the install prefix, e.g. `make install PREFIX=/usr/local`.

## Usage

```sh
./unclean-git-repos [flags] [root]
```

- `root` — directory to scan. Defaults to the current working directory.
- `-v` — also list clean repositories and skipped (non-git) folders.
- `-no-color` — disable colored output (also honors `NO_COLOR`).

Colors are automatically disabled when output is not a terminal.

### Example

```
$ ./unclean-git-repos
Scanning /home/me/projects

✗ 2 unclean repositories:

  api-service  (3 changes)
       M src/main.go
      A  src/new.go
      ?? notes.txt

  web-frontend  (1 change)
      ?? .env.local

Summary: 2 dirty, 4 clean, 1 skipped
(use -v to list clean repos and skipped folders)
```

## Exit codes

- `0` — no unclean repositories found.
- `1` — at least one unclean repository found (useful in scripts/CI).
- `2` — usage or I/O error.

## How it works

For each subdirectory the tool runs `git -C <dir> rev-parse --show-toplevel`
and only treats the folder as a repository if the reported root equals the
folder itself. This correctly skips plain folders that merely live inside a
parent git repository. Dirtiness is determined by `git status --porcelain`
(any staged, unstaged, or untracked change counts), which also flags brand-new
repositories that have files but no commits yet.
