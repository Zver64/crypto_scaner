# Issue tracker: Local Markdown

Issues and specs for this repo live as Markdown files in `.scratch/`.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`
- The spec is `.scratch/<feature-slug>/spec.md`
- Implementation issues are one file per ticket at `.scratch/<feature-slug>/issues/<NN>-<slug>.md`, numbered from `01`
- Never place all tickets in one combined file
- Triage state is recorded as a `Status:` line near the top of each issue
- Comments and conversation history are appended under `## Comments`

## Publishing and fetching

When a skill says “publish to the issue tracker”, create a file under `.scratch/<feature-slug>/`.

When a skill says “fetch the relevant ticket”, read the referenced issue file.

## Blocking dependencies

A `Blocked by: NN, NN` line records dependencies. A ticket is unblocked when every referenced ticket is complete.

## Wayfinding operations

- Map: `.scratch/<effort>/map.md`
- Child ticket: `.scratch/<effort>/issues/NN-<slug>.md`
- Ticket type: `Type: research`, `prototype`, `grilling`, or `task`
- Ticket state: `Status: claimed` or `resolved`
- Claim a ticket by setting `Status: claimed`
- Resolve it by adding `## Answer`, setting `Status: resolved`, and recording the result in the map
