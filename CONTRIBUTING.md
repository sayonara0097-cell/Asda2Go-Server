# Contributing

Thanks for improving AsdaGo. This project is a Go rewrite of the WCell Asda2 server, so changes should stay close to the existing protocol behavior unless there is a clear reason to diverge.

Before making code changes, read `DEVELOPMENT_RULES.md`. Those rules are part of this contribution guide and must stay current with the source.

## Development Environment

Use Visual Studio Code, not Visual Studio.

Recommended VS Code setup:

- Install the Go extension: `golang.Go`.
- Open this folder directly: `Current Source Code`.
- Let VS Code install `gopls` when prompted.
- Keep format-on-save enabled.

The workspace settings in `.vscode/settings.json` configure Go formatting through `gofmt` and enable common Go diagnostics.

## Code Style

Use one code style across the project:

- Run `gofmt` before committing.
- Follow `DEVELOPMENT_RULES.md` for handler ownership, folder placement, database direction, and WCell porting rules.
- Keep code idiomatic Go and avoid C# naming patterns unless the name is a protocol opcode or a direct WCell compatibility name.
- Prefer small, focused changes that preserve packet layouts from the WCell reference.
- Add short comments only where they explain protocol offsets, packet layouts, or non-obvious WCell behavior.

## Checks

Run these before sharing changes:

```powershell
gofmt -w .
go test ./...
go vet ./...
```

Optional linter:

```powershell
golangci-lint run
```

The repository includes `.golangci.yml` for consistent lint results when `golangci-lint` is installed.

## Project Layout

- `shared/packet`: PacketIn, PacketOut, and all opcode constants as the one source of truth.
- `shared/crypt`: Asda2 XOR tables and locale handling.
- `shared/db`: database access for accounts, characters, inventory, and related records.
- `shared/types`: shared client/account/character/inventory data structures.
- `shared/relay`: channel registry, login handoff, and future cross-channel message types.
- `cmd/loginserver`: auth, character select, character create, and enter-world redirect.
- `cmd/gameserver`: map/world runtime and gameplay handlers.
- `cmd/npcserver`: per-channel static NPC and monster visibility HTTP service.
- `cmd/relayserver`: relay/hub stub and status HTTP endpoints.
- `cmd/gmtool`: console GM tool that authenticates through relayserver and keeps GM commands out of game chat.

## Handler Split Rationale

Rather than one giant `handlers.go`, game handlers are split by domain matching how the original C# source is organized, for example `Asda2MovmentHandler.cs`, `Asda2ChatHandler.cs`, and item/combat handlers. This makes porting new handlers easy: look at the C# file name, find the matching `.go` file, and add it there.

Every new system should get a focused handler or module that matches its domain. Do not add NPC, skill, monster, item, social, mail, title, pet, chat-room, premium-item, or resurrection behavior into unrelated files. Avoid catch-all handler files; split small stubs by domain instead.

Weather work belongs in `cmd/gameserver/weather.go`, `cmd/gameserver/handlers_weather.go`, `shared/types/weather.go`, and `shared/db/weather.go`. Keep the packet layout aligned with `GlobalHandler.SendSetClientTimeResponse` from the WCell reference.

## Relay Foundation

`shared/relay/relay.go` defines the message types for inter-server communication, including `WorldAnnouncement`, `CrossChannelChat`, and `PlayerOnlineStatus`. The relay server in `cmd/relayserver` listens for game server connections, and each game server has a thin relay client that registers itself and sends heartbeat/player-count updates. World announcements can be posted through relay HTTP and are delivered to connected game servers, then forwarded to logged-in players with the WCell system/global chat packet (`GlobalChatWithItemResponse` / opcode 6561).

## Working With The WCell Reference

When porting behavior from the reference source:

- Cite the WCell file or handler in comments when packet offsets are hard to infer.
- Prefer matching packet field order exactly.
- For NPCs, skills, and monsters, port only the Asda2 functions that are actually needed; do not copy whole WoW-era systems.
- Keep legacy fallbacks only when the reference keeps them.
- Avoid broad rewrites while debugging login, world entry, movement, inventory, or combat.

## Database Rules

Database access currently belongs in `shared/db`. The current Asda2-only schema additions live in `shared/db/schema.sql`; keep new table definitions there until a dedicated migration layout exists. Do not add WoW-only tables or columns unless a current Asda2 packet/runtime system directly consumes them.

## Build

```powershell
.\scripts\build-windows.ps1
```

The script builds `loginserver.exe`, `relayserver.exe`, `gmtool.exe`, the base `gameserver.exe` and `npcserver.exe`, plus channel-named `gameserver-ch0.exe` through `gameserver-ch2.exe` and `npcserver-ch0.exe` through `npcserver-ch2.exe`.

GitHub Actions runs the same build through `.github/workflows/build-windows.yml`. The workflow can be started manually from the repository's Actions tab with **Run workflow**, and uploads the Windows binaries as the `asda2-windows-bin` artifact.

## Run

The login, game, NPC, and relay servers accept CLI bind addresses and matching env vars.

```powershell
.\bin\relayserver.exe -bind 127.0.0.1:5200 -http 127.0.0.1:7000
.\bin\npcserver.exe -bind 127.0.0.1:5300 -channel 0
.\bin\npcserver.exe -bind 127.0.0.1:5301 -channel 1
.\bin\npcserver.exe -bind 127.0.0.1:5302 -channel 2
.\bin\gameserver.exe -bind 0.0.0.0:5100 -channel 0 -server-id game-channel-0 -public-port 5100 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5300
.\bin\gameserver.exe -bind 0.0.0.0:5101 -channel 1 -server-id game-channel-1 -public-port 5101 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5301
.\bin\gameserver.exe -bind 0.0.0.0:5102 -channel 2 -server-id game-channel-2 -public-port 5102 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5302
.\bin\loginserver.exe -bind 0.0.0.0:5000 -channels 0=127.0.0.1:5100,1=127.0.0.1:5101,2=127.0.0.1:5102
```

Env examples live in `configs/`.

Useful relay status endpoints:

```powershell
Invoke-RestMethod http://127.0.0.1:7000/status
Invoke-RestMethod http://127.0.0.1:7000/gameservers
Invoke-RestMethod http://127.0.0.1:7000/channels
```

GM commands should be tested through the console tool, not through game chat:

```powershell
.\bin\gmtool.exe -server http://127.0.0.1:7000
```

The GM tool receives online players from the game server relay heartbeat. Keep player-list and GM-action changes in the relay/gmtool path, not in normal game chat handlers.

Monster work belongs in `cmd/gameserver/monsters.go` and `cmd/gameserver/handlers_monsters.go`. Keep it Asda2-only and port reference behavior in small verified slices.

Item work belongs in `shared/types/inventory.go`, `shared/types/item_template.go`, `shared/db`, `cmd/gameserver/items.go`, and focused item handler files such as `handlers_items.go`, `handlers_loot.go`, `handlers_shop_items.go`, and `handlers_repair.go`. Keep item metadata Asda2-only through `Asda2ItemTemplate` or static data, and port deeper WCell formulas in small verified slices instead of mixing them into unrelated systems.
