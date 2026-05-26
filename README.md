# Asda2 Go Rewrite

A Go rewrite of the WCell Asda2 emulator, organized as separate login, game, NPC, and relay servers.

Project rules for future contributors live in `DEVELOPMENT_RULES.md`; the short version is focused handlers, Asda2-only porting, one clean database direction, VS Code, `gofmt`, `go vet`, and optional `golangci-lint`.

## Layout

```text
asda2/
├── go.mod / go.sum
├── README.md
├── shared/
│   ├── packet/      # PacketIn, PacketOut, opcode constants
│   ├── crypt/       # Asda2 XOR locale tables
│   ├── db/          # Database access for accounts, characters, items
│   ├── types/       # Shared Account, Client, Character, inventory types
│   └── relay/       # Channel registry, handoff, relay message stubs
├── cmd/
│   ├── loginserver/ # Auth, character select, enter-world redirect
│   ├── gameserver/  # World runtime and gameplay handlers
│   ├── npcserver/   # NPC visibility/runtime HTTP service
│   ├── relayserver/ # Cross-channel relay stub, GM API, and status HTTP endpoints
│   ├── gmtool/      # Console GM tool that talks to relayserver HTTP
│   └── gameserver-npc-mobs/ # Exports static NPC/mob data from the Asda2 DB
└── configs/         # Env examples for each server
```

## Server Split

- Login server: authenticates, shows character select, creates characters, and sends the selected game channel endpoint.
- Game server: owns maps, movement, gameplay packets, and a configurable channel port.
- NPC server: owns static NPC spawn visibility over HTTP. The game server can use it with `ASDA2_NPCSERVER_ADDR` and will skip its local NPC spawn runtime.
- Relay server: foundation for future cross-channel chat, world announcements, and player online status.
- GM tool: authenticates against the `Account` table through relayserver, checks `RoleGroupName`, then exposes admin commands outside game chat.

The game exposes exactly 3 channels (`0`, `1`, and `2`). Each channel is owned by its own game server process and should point at its own NPC server process. Game servers write heartbeats to `ServerChannel`. Login reads that registry before sending `EnterWorldIpeResponse`. Login-to-game and channel-change reconnect validation use `ServerHandoff`.

## Handler File Split Rationale

Rather than one giant `handlers.go`, game handlers are split by domain matching how the original C# source is organized, for example `Asda2MovmentHandler.cs` maps to `handlers_movement.go`, combat maps to `handlers_combat.go`, skills map to `handlers_skills.go`, and focused systems such as mail, pets, titles, chat rooms, premium items, and resurrection get their own files. This makes porting new handlers easy: look at the C# file name, find the matching `.go` file, and add it there.

## Cross-Channel Relay Foundation

`shared/relay/relay.go` defines message types for inter-server communication: `WorldAnnouncement`, `CrossChannelChat`, and `PlayerOnlineStatus`. The relay server in `cmd/relayserver` listens for game server connections. Each game server has a thin relay client that registers itself and sends heartbeat/player-count updates on startup. World announcements can already be broadcast from the relay HTTP endpoint to connected game servers, and each game server forwards them to logged-in players with the WCell system/global chat packet (`GlobalChatWithItemResponse` / opcode 6561).

## Build

```powershell
.\scripts\build-windows.ps1
```

The build script writes these Windows executables to `bin/`:

- `loginserver.exe`
- `gameserver.exe`, `gameserver-ch0.exe`, `gameserver-ch1.exe`, `gameserver-ch2.exe`
- `npcserver.exe`, `npcserver-ch0.exe`, `npcserver-ch1.exe`, `npcserver-ch2.exe`
- `relayserver.exe`
- `gmtool.exe`

The `-ch0`, `-ch1`, and `-ch2` filenames are intentional. The game and NPC servers can infer their default channel from the executable name.

## GitHub Build Button

This repository includes a GitHub Actions workflow at `.github/workflows/build-windows.yml`. In GitHub, open the repository, go to **Actions**, choose **Build Windows Binaries**, then press **Run workflow**. When it finishes, download the `asda2-windows-bin` artifact from the workflow run.

## Run

Each server reads its bind address from a CLI arg or env var.

```powershell
.\bin\relayserver.exe -bind 127.0.0.1:5200 -http 127.0.0.1:7000
.\bin\npcserver.exe -bind 127.0.0.1:5300 -channel 0
.\bin\npcserver.exe -bind 127.0.0.1:5301 -channel 1
.\bin\npcserver.exe -bind 127.0.0.1:5302 -channel 2
.\bin\gameserver.exe -bind 0.0.0.0:5100 -channel 0 -server-id game-channel-0 -public-ip 127.0.0.1 -public-port 5100 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5300
.\bin\gameserver.exe -bind 0.0.0.0:5101 -channel 1 -server-id game-channel-1 -public-ip 127.0.0.1 -public-port 5101 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5301
.\bin\gameserver.exe -bind 0.0.0.0:5102 -channel 2 -server-id game-channel-2 -public-ip 127.0.0.1 -public-port 5102 -relay 127.0.0.1:5200 -npc-server http://127.0.0.1:5302
.\bin\loginserver.exe -bind 0.0.0.0:5000 -public-ip 127.0.0.1 -game-port 5100 -channels 0=127.0.0.1:5100,1=127.0.0.1:5101,2=127.0.0.1:5102
```

Equivalent env files live in `configs/`.

## Relay Status

```powershell
Invoke-RestMethod http://127.0.0.1:7000/health
Invoke-RestMethod http://127.0.0.1:7000/status
Invoke-RestMethod http://127.0.0.1:7000/gameservers
Invoke-RestMethod http://127.0.0.1:7000/channels
Invoke-RestMethod http://127.0.0.1:7000/handoffs
```

World announcements are sent through the authenticated GM tool.

## GM Tool

Set your GM account role in the `Account.RoleGroupName` column. Default allowed roles are `GM`, `Developer`, `Admin`, `Owner`, and `GameMaster`. You can override them with:

```powershell
$env:ASDA2_GM_ROLES="GM,Developer,Admin,Owner"
```

Run the tool while `relayserver` is running:

```powershell
.\bin\gmtool.exe -server http://127.0.0.1:7000
```

After login, GM commands are selected from a menu. `Online players`, `World announcement`, `Teleport player`, `Summon monster`, and `Kill monster` are active. The monster runtime is intentionally small and Asda2-focused; deeper NPC AI, loot, and skill behavior should be ported in focused slices.

## Map Registry

Asda2 map IDs are centralized in `shared/types/maps.go`. `MapId.Silaris` is `0` in the original C# source; the similarly named zone id is `ZoneId.SilarisMain = 4988`. The Go runtime accepts either form and normalizes zone ids such as `4988` back to their real `MapId` before entering the world, teleporting, or calculating local coordinates.

## Base Stats

Base HP, MP, and class attributes live in `Asda2BaseStat`. On an empty table, loginserver or gameserver seeds it from the editable reference files in `../BaseStats` (`BASE_STATS_REFERENCE.txt` and `seed_class_attributes.sql`). Set `ASDA2_BASE_STATS_DIR` if the folder is elsewhere, and set `ASDA2_BASE_STATS_RESEED=1` when you intentionally want to reload changed work-in-progress values into the database.

## Static World Data

The game server prefers static world data from `data/asda2`:

- `data/asda2/monsters/templates.json`: monster ids, names, level, HP, and movement speed.
- `data/asda2/monsters/maps/*.json`: per-map spawn groups, points, AI labels, respawn time, and loot table names.
- `data/asda2/monsters/drops.json`: drop templates grouped by monster id.
- `data/asda2/monsters/movement_paths.json`: DB waypoint paths grouped by creature spawn id.
- `data/asda2/npcs/templates.json`: static non-combat NPC/game-object ids, names, and kind.
- `data/asda2/npcs/maps/*.json`: per-map NPC spawn groups and points, preserving DB `guid` values as stable `spawnId` values.
- `data/asda2/npcs/vendors.json`: optional regular-shop stock grouped by NPC template id.

Refresh monster static data from the canonical Asda2 database with:

```powershell
go run ./cmd/gameserver-npc-mobs -out data/asda2
```

The exporter is the `gameserver-npc&mobs` tool. Its Go source directory uses `gameserver-npc-mobs` because Go import paths cannot contain `&`; when you want a standalone executable, build it as:

```powershell
go build -o 'bin\gameserver-npc&mobs.exe' ./cmd/gameserver-npc-mobs
```

The exporter reads `creature_template`, `creature`, `creature_movement`, `asda2itemdroptemplate`, `gameobject_template`, `gameobject`, and optional vendor stock from `Asda2NpcVendorItem` or `RegularShopRecord`. It preserves DB `guid` values as static `spawnId` values so client targeting remains stable.

Set `ASDA2_WORLD_DATA_DIR` when the data folder lives somewhere else. If static files are missing, the server falls back to the legacy DB tables (`Asda2MonsterTemplate`, `Asda2MonsterSpawn`, `Asda2NpcTemplate`, `Asda2NpcSpawn`). Monster spawns can be refreshed through the GM Tool with `Reload monster spawns`, which reloads the static files when present. When a static monster is killed, the game server sends `MonstrStateChanged`, removes it from runtime, then respawns it after `RespawnSeconds`.

The first AI slice is intentionally small: monsters acquire nearby players, chase with `MonstMove`, return to their home spawn when leashed, and send simple non-lethal attack pulses. Player attacks now accept live monster session IDs, use `StartAtackResponse` to start the client attack animation, then apply damage on `ContinueAtack` pulses with `MonstrTakeDmg`, while still using the same death/respawn runtime. Monster kills now create loot with `ItemDroped`; `PickUpItem` removes it from the world and credits gold or creates a persistent `Asda2Item` in the character inventory.

## Weather

Weather is owned by the game server in `cmd/gameserver/weather.go` and `cmd/gameserver/handlers_weather.go`. The runtime loads optional per-map rows from the Asda2-only `Asda2Weather` table; missing rows mean clear weather. `Channel = -1` applies to every channel, while a channel-specific row overrides the shared row for that map.

`SetClientTime` mirrors the weather-focused WCell reference: it sends 6 payload bytes for client time, reserved byte, weather type, and encoded weather level. The game server sends it during login and every 10 seconds only when the in-game client-time value changes.

## Item Runtime

The game server initializes an Asda2 item runtime from optional `Asda2ItemTemplate` rows. When an item has no template row yet, the server uses a conservative fallback template so existing DB items and monster drops still work. `Asda2Item` remains the persistent owner/slot/state table; `Asda2ItemTemplate` is the Asda2-only reference table for item kind, stackability, weight, price, durability, required level/profession, equipment slot, and sowel socket count.

Implemented item flows include inventory load and login packets, fast item slots, monster loot pickup into inventory, item move/swap/equip packets, delete, buy, sell, basic use, warehouse/avatar warehouse transfer, sowel insert/remove, repair, basic upgrade, recipe learn, and a minimal recipe-id-to-result crafting path. Deeper formulas such as full WCell stat generators, advanced enchant probabilities, and complete vendor recipe data should be ported in focused slices from the Asda2 reference files.

## Development Checks

Use VS Code, `gofmt`, and the included linter config:

```powershell
gofmt -w .
go test ./...
go vet ./...
golangci-lint run
```
