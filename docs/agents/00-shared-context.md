# Shared Agent Context

You are working on AsdaGo, a focused Go rewrite of the Asda2 server.

Reference source, read-only:

```text
<reference-source-root>
```

Active Go source, editable:

```text
<active-go-source-root>
```

Before changing code, read:

- `AGENTS.md`
- `DEVELOPMENT_RULES.md`
- `CONTRIBUTING.md`

Rules:

- Port only Asda2 behavior.
- Do not bulk-copy WCell or WoW-era systems.
- Preserve packet field order exactly.
- Keep each gameplay system in its own focused handler file.
- Put shared code under `shared/`.
- Keep schema work centralized in `shared/db/schema.sql`.
- Prefer small verified feature slices.
- Use `rg` for source search.
- Run `gofmt -w .`, `go test ./...`, and `go vet ./...` before marking implementation done.

Feature:

```text
<fill feature/system here>
```
