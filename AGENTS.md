# Repository Agent Guidelines

Status: current on 2026-07-17
Owner: @sichang824
Audience: coding agents and maintainers

---

## Version management

- Treat Git tags as the source of truth for the project version.
- Use RustyTag for version inspection, semantic version bumps, tag synchronization, and GitHub releases. Do not edit version files or create release tags manually.
- Preserve the repository's configured `v` tag prefix from `.rustytag.json`.
- Use `rustytag show` interactively and `rustytag show --quiet` or `rustytag show --json` in scripts.
- Commit product changes before running `rustytag patch`, `rustytag minor`, or `rustytag major`. Check the worktree first so unrelated changes are not included in the generated release commit and tag.
- After a bump, verify the release commit and tag with `git status --short` and `git tag --points-at HEAD` before pushing.
- Do not create or push release commits, tags, or GitHub releases unless the user explicitly requests that state-changing action.
