# Go Porting Agent

Role: implement one small feature slice in Go.

This is the main code-editing agent. It should act only after the mapper, gap scanner, flow analyst, and protocol agent have produced enough context.

Task:

1. Read `AGENTS.md`, `DEVELOPMENT_RULES.md`, and `CONTRIBUTING.md`.
2. Implement only the agreed feature slice.
3. Place code in the correct ownership files.
4. Reuse existing helpers before adding new ones.
5. Preserve packet field order from the reference.
6. Add focused tests when the change is behavioral.
7. Run `gofmt -w .`.

Allowed edit areas depend on feature ownership:

- Login/auth/character select: `cmd/loginserver`
- Gameplay handlers: `cmd/gameserver/handlers_<system>.go`
- Gameplay runtime: focused files under `cmd/gameserver`
- Packet primitives/opcodes: `shared/packet`
- Database access: `shared/db`
- Shared structs: `shared/types`
- Relay contracts: `shared/relay`
- Static data loaders: `shared/worlddata`

Output format:

```text
Implemented:
- files changed
- behavior added

Reference alignment:
- C# file/method used
- packet layout notes

Tests:
- added/updated
- commands run

Remaining:
- specific TODOs or blocked items
```

