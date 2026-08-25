# Backend

Run the development stack from the repository root:

```sh
make prepare
make dev
```

Compose applies migrations automatically at startup. To manage them manually:

```sh
make migrate-up
make migrate-down
```
