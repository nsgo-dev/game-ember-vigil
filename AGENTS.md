# Ember Vigil

A twelve-minute survivors-like 2D game in Go with NSGo and nsgo-dev/game. Read [ARCHITECTURE.md](ARCHITECTURE.md) before coding, then the relevant subject.

## Principles

- Keep the run in `run.go` headless, deterministic and free of NSGo views; the shell translates input, draws and saves.
- Keep balance in `data/*.yaml`; a view never holds a rule or a tuning number.
- Keep procedural ground, actors and particles in code as fallback (with lightweight particles drawn procedurally); stylized 2D sprite sheets, arena environment art, card icons, app icon and title key art live under `assets/` and are designed by 彩叶.
- Pin the game layer and NSGo. Ask before changing either dependency, adding one or changing a public API.
- Add no CI until the user asks for it.

## Working with the user

- Do not create sub-Agents or Git worktrees; work in the current checkout.
- Claim an issue by assigning it to the user, pushing an empty commit and opening a draft PR that names it, before implementing.
- Ask before renaming the repository or changing its settings.
- Commit, push or open pull requests only when the user asks; the request that starts an issue task covers its branch and pull request.
- Write issue titles and bodies in English.
- Test on the user's Linux machine (`ssh linux`) and Windows machine (`ssh pc`) when a change needs them.

## Verification

- Run `go tool nsgo test ./...` and `go tool nsgo check -target macos,windows,linux,web`, as [README Development](README.md#development) says.
- After changing the run or its data, check that the same seed and controls replay the same run and that every data file is canonical.

## Documentation

- Update a document only when a fact its reader needs changes.
- Write only what code and comments cannot show: purpose, boundaries and reasons.
- Keep each document within its budget; trim it before handing off when it is over.
- Keep `README.md` and `README.zh-CN.md` equivalent; everything else, including code comments, is English.
- Record user-visible outcomes as one-clause `Unreleased` bullets in `CHANGELOG.md`, crediting each pull request's author.
- Repair documents you touch; link to the owner of a fact instead of repeating it.
