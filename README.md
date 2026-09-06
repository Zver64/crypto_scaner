# Crypto Scanner

Telegram Mini App: Go, React and PostgreSQL.

```sh
cp .env.example .env
make prepare
make dev
```

Compose starts the development stack in watch mode and applies PostgreSQL migrations automatically. On normal backend startup, `ADMIN_TELEGRAM_ID` is inserted as an administrator only when no row for that Telegram ID exists. Existing rows—including disabled administrators—are left unchanged; the removed `bootstrap-admin` command is replaced by this automatic insert-only startup provisioning.

`make prepare` installs backend and frontend dependencies and the Git hooks.
Run the full verification suite with:

```sh
make check
```

## Docker Compose operations

`compose.production.yaml` is a standalone production deployment file, not an
overlay for `compose.yaml`. Set `BACKEND_IMAGE` and `FRONTEND_IMAGE`, along with
the required PostgreSQL settings, before deploying it.

Production services use Docker's `local` logging driver with rotation (10 MiB
per file, three files) to bound container log disk use. The backend gets a
30-second Compose stop grace period by default; override
`BACKEND_STOP_GRACE_PERIOD` only with a value greater than `SHUTDOWN_TIMEOUT`
(the application default is 15 seconds).

See Docker's references for
[stop grace periods](https://docs.docker.com/reference/compose-file/services/#stop_grace_period),
[logging configuration](https://docs.docker.com/engine/logging/configure/), and
[Compose interpolation](https://docs.docker.com/reference/compose-file/interpolation/).

## Git hooks

Lefthook is a repository-local Go tool, declared in [`.tools/go.mod`](.tools/go.mod)
and configured at the root in [`lefthook.yml`](lefthook.yml). Run `make prepare`
after cloning to install the configured Git hooks. They cover both `backend/`
and `frontend/`; Lefthook reloads its configuration for each hook run.

## Releases

Production deployments are immutable releases. Release Please creates and
updates a release PR from Conventional Commit messages merged into `main`:
`fix:` makes a patch release, `feat:` makes a minor release, and `!` or a
`BREAKING CHANGE:` footer makes a major release. Merging that release PR creates
the GitHub release and tag, then deploys both images under that exact tag.

Local commits are checked by Lefthook and Conform. Pull request titles are
checked by the `Conventional Commit` GitHub Actions workflow. Configure GitHub
to allow squash merges and use the PR title for the squash-commit title; make
the `Conventional Commit / Validate PR title` check required for `main`.

The release workflow uses `RELEASE_PLEASE_TOKEN` when configured, then falls
back to `GITHUB_TOKEN`. A fine-grained repository token with read/write access
to contents, pull requests, and issues is required for Release Please-generated
pull requests to trigger the required workflow.

## Migrations

```sh
make migrate-up    # apply migrations manually
make migrate-down  # roll back one migration manually
```
