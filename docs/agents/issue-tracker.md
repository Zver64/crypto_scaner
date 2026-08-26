# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- Create an issue with `gh issue create`.
- Read an issue with `gh issue view <number> --comments`.
- List issues with `gh issue list`, using appropriate label and state filters.
- Comment with `gh issue comment <number>`.
- Apply or remove labels with `gh issue edit`.
- Close issues with `gh issue close`.

Infer the repository from `git remote -v`; `gh` does this automatically inside the clone.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## Publishing and fetching

When a skill says “publish to the issue tracker,” create a GitHub issue.

When a skill says “fetch the relevant ticket,” run `gh issue view <number> --comments`.

## Wayfinding operations

- A map is one issue labeled `wayfinder:map`.
- Child tickets are GitHub sub-issues where available, with task-list links as a fallback.
- Ticket labels use `wayfinder:<type>`.
- Blocking relationships use GitHub issue dependencies, with a `Blocked by:` line as fallback.
- Claim a ticket by assigning it to the active developer.
- Resolve it by recording the answer, closing the ticket, and updating the map.
