# Domain Docs

This repository uses a single-context domain documentation layout.

## Before exploring

Read when present:

- `CONTEXT.md` at the repository root
- relevant ADRs under `docs/adr/`

If these files do not exist, proceed silently. Domain-modeling workflows create them when terminology or architectural decisions need to be recorded.

## Layout

```text
/
├── CONTEXT.md
├── docs/
│   └── adr/
└── backend/
    └── internal/
```

## Vocabulary

Use domain terms as defined in `CONTEXT.md`. Avoid introducing synonyms for an already defined concept.

If a required concept is absent, reconsider whether new terminology is necessary or record the gap for domain modeling.

## ADR conflicts

If proposed work contradicts an existing ADR, identify the conflict explicitly instead of silently overriding the decision.
