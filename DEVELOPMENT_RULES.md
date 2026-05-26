# Development Rules

These rules are mandatory for every future change to AsdaGo. They keep the rewrite focused on Asda2, easy to review, and safe for outside contributors when the source is published.

## Core Rules

- Keep code style unified across all server parts. Use the existing Go patterns, shared packages, naming conventions, `gofmt`, `go vet`, and the repository linter config.
- Keep each gameplay system in its own handler file. For example, movement belongs in `handlers_movement.go`, combat belongs in `handlers_combat.go`, skills belong in `handlers_skills.go`, and focused systems such as mail, titles, pets, chat rooms, premium items, and monsters must have their own handler files. Avoid catch-all files such as `handlers_misc.go`.
- Do not bulk-copy WCell or WoW-era systems. For NPCs, skills, monsters, and other large systems, inspect the reference carefully and port only the functions, packet fields, and data that Asda2 actually needs.
- Keep contribution rules current. Any workflow, layout, build, lint, or verification change must be reflected in `CONTRIBUTING.md`.
- Keep database schema ownership in one place. The project should move toward a single clean Asda2-only SQL schema file that can ship with the source; avoid scattering schema definitions across unrelated files.
- Keep code clean and purposeful. Each file, type, function, and handler should contain only the logic needed for its system to work, with short comments only for protocol offsets, packet layouts, or non-obvious Asda2/WCell behavior.
- Put every file in the directory that matches its responsibility. Shared code goes in `shared/`, server entrypoints and server-only handlers go under `cmd/<server>/`, and configuration examples go in `configs/`.
- Remove dead duplication. Before adding new helpers, check whether an equivalent already exists; if old code no longer serves a purpose, delete or consolidate it as part of the same focused change.

## Handler Ownership

- `cmd/loginserver`: login, account auth, character select/create, and login-to-game handoff only.
- `cmd/gameserver`: live world state and gameplay handlers only.
- `cmd/gameserver/handlers_channels.go`: game channel change requests, channel-bound state reset, and channel reconnect packets only.
- `cmd/npcserver`: per-channel NPC/monster visibility service only; one process owns exactly one game channel.
- `cmd/gameserver/loot.go` and `cmd/gameserver/handlers_loot.go`: loot runtime, world drops, and pickup packet handling only.
- `cmd/gameserver/weather.go` and `cmd/gameserver/handlers_weather.go`: map weather runtime, client-time/weather sync, and weather packet layout only.
- `cmd/relayserver`: relay/hub state, cross-channel messages, status HTTP, and authenticated GM API only.
- `cmd/gmtool`: external GM/operator UI only. GM actions should not be implemented as in-game chat commands.
- `shared/packet`: packet builders/readers and opcode constants only.
- `shared/db`: database access only. Keep SQL centralized here until a dedicated schema/migration layout exists.
- `shared/relay`: inter-server message contracts and thin relay client helpers only.
- `shared/types`: shared data structures that are used by multiple servers.

## Porting From Reference Source

- Start from the Asda2 handler name in the C# reference, then place the Go implementation in the matching handler file.
- Preserve packet field order exactly and document tricky offsets with a short source reference.
- Prefer a small working slice over a large incomplete port.
- If a WCell function mixes WoW behavior with Asda2 behavior, extract only the Asda2 path.
- Do not introduce broad abstractions until at least two real systems need them.

## Database Direction

- Current DB access lives in `shared/db`.
- New schema work should target one Asda2-only SQL file, with table definitions, indexes, and seed/reference rows organized by system.
- Base stat values must be read from `Asda2BaseStat`; do not hardcode class or level stats in gameplay handlers. The editable source files live in the top-level `BaseStats` folder and can reseed the DB intentionally.
- Item metadata should come from Asda2-only item template data such as `Asda2ItemTemplate`; fallback item classification is only a compatibility layer for old DB rows and exported monster drops until full template data is seeded.
- Avoid adding WoW-only tables or columns unless an Asda2 packet or runtime system directly needs them.
- Any temporary compatibility fallback must be marked clearly and removed once the Asda2 schema is stable.

## Required Checks

Run these before sharing changes:

```powershell
gofmt -w .
go test ./...
go vet ./...
```

Run the linter when installed:

```powershell
golangci-lint run
```

Use Visual Studio Code for the development environment, not Visual Studio.
