# AsdaGo Agent Operating Guide

This project is a focused Go rewrite of the Asda2 server.

Use this file when working with coding agents from VS Code or any local assistant. The goal is to keep agent work fast, precise, and safe for packet-heavy game-server porting.

## Source Roles

- Reference source, read-only:
  `<reference-source-root>`
- Active Go source, editable:
  `<active-go-source-root>`

## Non-Negotiable Rules

Every agent must read these files before changing code:

- `DEVELOPMENT_RULES.md`
- `CONTRIBUTING.md`

Core rules:

- Port only Asda2 behavior from the WCell reference.
- Do not bulk-copy WCell or WoW-era systems.
- Preserve packet field order exactly.
- Keep gameplay systems in their focused handler files.
- Keep shared code under `shared/`.
- Keep database schema work centralized in `shared/db/schema.sql`.
- Prefer small verified feature slices over large incomplete ports.
- Run `gofmt`, `go test ./...`, and `go vet ./...` before marking code done.

## Agent Team

Use all agents, but do not let all agents edit code.

- Reference Mapper: reads C# reference and maps source functions, packet handlers, packet writers, enums, DB records, and cross-calls.
- Go Gap Scanner: reads Go source and identifies existing implementation, TODOs, handlers, tests, and likely edit locations.
- Flow Analyst: explains the actual Asda2 flow and filters out WCell/WoW-only behavior.
- Packet/Protocol Agent: owns opcode matching, packet reads/writes, offsets, response packet order, and golden packet tests.
- Go Porting Agent: performs the actual Go implementation for one small feature slice.
- Structure Guardian: checks file ownership, duplicated helpers, schema placement, naming, comments, and documentation updates.
- Test/Runtime Agent: adds tests, runs verification commands, builds/runs servers when needed, and reviews logs.
- Data/DB Agent: owns schema, static data, templates, seed/reference rows, and Asda2-only data direction.
- Regression Reviewer: reviews completed changes for bugs, missing tests, packet mismatches, and ownership violations.
- Related Domain Specialist Agent(s): system-specific reviewers used only when the feature touches their domain. These agents should usually analyze and guide before implementation; the Go Porting Agent remains the only default code-editing agent.

## Mandatory Pipeline

Whenever agents are used for a feature/system, they must follow this order:

```text
Reference Mapper
-> Go Gap Scanner
-> Flow Analyst
-> Packet/Protocol Agent
-> Related Domain Specialist Agent(s), if related
-> Go Porting Agent
-> Structure Guardian
-> Test/Runtime Agent
-> Data/DB Agent, when data/schema is involved
-> Regression Reviewer
```

This sequence is mandatory for systems such as inventory, skills, pets, soulmate, mail, battleground, NPCs, monsters, combat, quests, trade, and private shop.

Only one agent should edit code for a feature at a time. The other agents should map, analyze, verify, review, or test.

If a feature clearly does not need data or schema changes, the Data/DB Agent still makes a short "not needed" note instead of doing full schema work.

## Coordination Rule

Use the mandatory pipeline above. Do not skip directly from source mapping to implementation unless the user explicitly asks for a quick prototype.

Canonical sequence:

```text
Reference Mapper
-> Go Gap Scanner
-> Flow Analyst
-> Packet/Protocol Agent
-> Related Domain Specialist Agent(s), if related
-> Go Porting Agent
-> Structure Guardian
-> Test/Runtime Agent
-> Data/DB Agent, when data/schema is involved
-> Regression Reviewer
```

## Related Domain Specialist Agents

Use these only when the feature touches the matching system. If none are related, make a short "no related domain specialist needed" note and continue the mandatory pipeline.

### Foundation Domain Agents

- Packet/Opcode/Encryption Agent: use for packet layout, opcode constants, packet reader/writer behavior, Asda2 XOR crypt, locale string encoding, router registration, or golden packet tests.
- Login/Character Flow Agent: use for account auth, sessions, character list/create/select, handoff tokens, login-to-game redirect, duplicate sessions, and character initialization.
- World/Map/Movement/Visibility Agent: use for world runtime, map registry, movement, position validation, WhoIsHere, map change, player visibility, and channel-bound world state.
- Static World Data/NPC/Monster/NpcServer Agent: use for `data/asda2`, `shared/worlddata`, npcserver visibility, static NPC/monster templates, spawns, drops, movement paths, and npcserver HTTP sync.
- Inventory Agent: use for character inventory state, slots, bags, equipment slots, stack movement, item deletion, inventory packet behavior, and inventory persistence.
- Item Template/Item Use Agent: use for Asda2 item templates, item classification, item metadata, item usage, consumables, requirements, and compatibility fallbacks.
- Loot Agent: use for monster drops, world drops, pickup flow, loot ownership, loot packets, and `cmd/gameserver/loot.go`.
- Repair Items Agent: use for durability repair, repair cost, repair packets, and item durability status.
- Shop Items Agent: use for NPC shop/vendor buy/sell, shop item packets, shop pricing, and vendor stock.
- Item Upgrade Agent: use for upgrade/enchant/disassemble/crafting-related item upgrade behavior, item options, and upgrade result packets.
- Character Stats Agent: use for base stats, derived stats, HP/MP, stat packets, equipment stat application, and level/stat recalculation.
- Profession/Skills Agent: use for Warrior/Archer/Mage profession behavior, learned skills, skill templates, skill packets, profession restrictions, and profession progression.
- Combat Agent: use for player combat state, normal attacks, skill attacks, player health, damage packets, resurrection, and PvP hooks.
- Monster Combat Agent: use for monster aggro, monster attacks, death, respawn, monster movement during combat, loot generation, and NPC/monster runtime combat events.
- Teleport/Portals/Map-Change Agent: use for teleport crystals, scrolls, bind locations, portals, map transitions, channel reconnect impact, and related packets.
- Weather Agent: use for in-game weather, client time sync, map weather state, and weather packets.
- Trade/Private Shop Agent: use for player trade, trade window, item/gold exchange, private shop open/buy/close, cancellation, and persistence.
- Guild Agent: use for guild creation, ranks, members, guild packets, guild DB, guild storage, guild war, and guild wave behavior.
- Social/Chat Agent: use for normal chat, whispers, party, friends, chat rooms, cross-channel social state, and social packet delivery.
- NPC Interaction/Vendor/Trainer Agent: use for NPC targeting, dialogue, vendors, trainers, quest givers, shops, and NPC interaction packets.
- Relay/GM Tool Agent: use for relay contracts, game-server registry, channel status, handoffs, world announcements, GM API, GM permissions, and gmtool behavior.

### Depth/Advanced Domain Agents

- Inventory Depth Agent: use for equipment stats, requirements, durability, all stack/equip/use/delete edge cases, full Asda2 item formulas, and item-loss/duplication risk.
- Skills Depth Agent: use for complete effects, buffs/debuffs, cooldowns, cast/range/resource validation, SoulGuard skills, and Soulmate skills.
- Combat Depth Agent: use for full damage/defense formulas, PvP, advanced status effects, real-client combat packet validation, and combat regression risks.
- NPC Systems Depth Agent: use for dialogue, trainers, vendors, quest givers, shops, spawned-NPC behavior beyond visibility, and NPC ownership boundaries.
- Social Systems Depth Agent: use for party, friends, whispers, chat rooms, cross-channel social state, relay-backed online status, and social persistence.
- Guild Systems Depth Agent: use for full guild creation, ranks, members, guild storage, guild permissions, guild war, and guild wave.
- Trade/Private Shop Depth Agent: use for full item/gold exchange validation, cancellation, rollback, persistence, private shop client edge cases, and duplication/loss prevention.
- Quests/Tutorial/Titles/Fishing/Digging Agent: use for quest handlers, tutorial flow, title discovery/getting, fishing, digging, and related reward/progression packets.
- Pets/Mounts/Soulmate/Mail/Battleground Agent: use for pet lifecycle, mounts, soulmate relationship/skills/messages, mail mailbox/send/read/delete, battleground registration/war/PvP systems, and existing opcode stubs.

Domain specialists should output:

```text
Related domain:
Why this agent is needed:
Relevant Go files:
Relevant C# reference files:
Implementation constraints:
Tests/runtime checks to require:
Risks:
```

## Current System Coverage Map

This snapshot helps agents see both the completed foundations and the remaining high-value gaps. "Done" here means a working foundation or implemented slice exists in the active Go source; it does not mean the system is 100% equal to the C# reference. Every agent must still verify the current code before editing.

### Done Or Established Foundations

- Project/server split:
  - `cmd/loginserver`
  - `cmd/gameserver`
  - `cmd/npcserver`
  - `cmd/relayserver`
  - `cmd/gmtool`
  - `cmd/gameserver-npc-mobs`
- Packet, opcode, and encryption foundation:
  - `shared/packet`
  - `shared/crypt`
  - `cmd/gameserver/router.go`
  - `cmd/loginserver/router.go`
- Login and character flow:
  - `cmd/loginserver`
  - `cmd/gameserver/login.go`
  - `cmd/gameserver/account_sessions.go`
  - `cmd/loginserver/account_sessions.go`
- World, map, movement, and visibility foundation:
  - `cmd/gameserver/world.go`
  - `cmd/gameserver/world_entities.go`
  - `cmd/gameserver/movement.go`
  - `cmd/gameserver/handlers_movement.go`
  - `cmd/gameserver/visibility.go`
  - `cmd/gameserver/map_change.go`
- Static world data, NPCs, monsters, and npcserver visibility:
  - `shared/worlddata`
  - `shared/npcruntime`
  - `cmd/npcserver`
  - `cmd/gameserver/npcs.go`
  - `cmd/gameserver/monsters.go`
  - `cmd/gameserver/npcserver_client.go`
  - `cmd/gameserver/npcserver_visibility.go`
  - `data/asda2`
- Inventory foundation:
  - `shared/types/inventory.go`
  - `shared/db/items.go`
  - `shared/db/item_templates.go`
  - `cmd/gameserver/items.go`
  - `cmd/gameserver/handlers_items.go`
- Item templates and item use foundation:
  - `shared/types/item_template.go`
  - `shared/db/item_templates.go`
  - `cmd/gameserver/item_use.go`
  - `cmd/gameserver/handlers_items.go`
- Loot foundation:
  - `cmd/gameserver/loot.go`
  - `cmd/gameserver/handlers_loot.go`
- Repair items foundation:
  - `cmd/gameserver/handlers_repair.go`
- Shop items foundation:
  - `cmd/gameserver/handlers_shop_items.go`
- Item upgrade foundation:
  - `cmd/gameserver/handlers_item_upgrade.go`
  - `cmd/gameserver/item_options.go`
- Character stats foundation:
  - `shared/db/base_stats.go`
  - `shared/stats`
  - `cmd/gameserver/character_stats.go`
  - `cmd/gameserver/experience.go`
- Profession, Warrior/Archer/Mage, and skills foundation:
  - `cmd/gameserver/professions.go`
  - `cmd/gameserver/quest_professions.go`
  - `cmd/gameserver/skills.go`
  - `cmd/gameserver/skill_packets.go`
  - `cmd/gameserver/skills_effects.go`
  - `cmd/gameserver/skills_soulguard.go`
  - `cmd/gameserver/skills_soulmate.go`
  - `cmd/gameserver/handlers_skills.go`
  - `shared/db/skills.go`
- Combat foundation:
  - `cmd/gameserver/combat_normal.go`
  - `cmd/gameserver/player_health.go`
  - `cmd/gameserver/handlers_combat.go`
  - `cmd/gameserver/handlers_resurrection.go`
- Monster combat foundation:
  - `cmd/gameserver/monsters.go`
  - `cmd/gameserver/handlers_monsters.go`
  - `cmd/gameserver/loot.go`
- Teleport, portals, and map-change foundation:
  - `cmd/gameserver/handlers_teleport.go`
  - `cmd/gameserver/portals.go`
  - `cmd/gameserver/map_change.go`
  - `shared/db/teleport.go`
  - `shared/db/portals.go`
- Weather foundation:
  - `cmd/gameserver/weather.go`
  - `cmd/gameserver/handlers_weather.go`
  - `shared/types/weather.go`
  - `shared/db/weather.go`
- Trade and private shop foundations:
  - `cmd/gameserver/trade_runtime.go`
  - `cmd/gameserver/handlers_trade.go`
  - `cmd/gameserver/private_shop_runtime.go`
  - `cmd/gameserver/handlers_private_shop.go`
- Guild foundation:
  - `cmd/gameserver/guild_runtime.go`
  - `cmd/gameserver/guild_logic.go`
  - `cmd/gameserver/guild_packets.go`
  - `cmd/gameserver/guild_handlers.go`
  - `shared/db/guilds.go`
- Social/chat foundation:
  - `cmd/gameserver/social_runtime.go`
  - `cmd/gameserver/social_packets.go`
  - `cmd/gameserver/handlers_chat.go`
  - `cmd/gameserver/handlers_friends.go`
  - `cmd/gameserver/handlers_party.go`
  - `shared/db/social.go`
- NPC interaction/vendor/trainer foundation:
  - `cmd/gameserver/npc_interactions.go`
  - `cmd/gameserver/npc_trainers.go`
  - `cmd/gameserver/npc_vendors.go`
  - `cmd/gameserver/handlers_npc_dialogue.go`
  - `cmd/gameserver/handlers_npc_trainers.go`
  - `cmd/gameserver/handlers_npc_vendors.go`
  - `shared/db/npc_vendors.go`
  - `shared/db/npcs.go`
- Relay and GM tool foundation:
  - `shared/relay`
  - `cmd/relayserver`
  - `cmd/gmtool`
  - `cmd/gameserver/gm.go`
  - `cmd/gameserver/gm_*.go`

### Partial Systems That Need Deeper Reference Alignment

- Inventory depth: equipment stats, requirements, durability, all stack/equip/use/delete edge cases, full Asda2 item formulas.
- Skills depth: complete effects, buffs/debuffs, cooldowns, range/resource validation, SoulGuard and Soulmate skill behavior.
- Combat depth: complete formulas, PvP, advanced status effects, real-client packet validation.
- NPC systems: dialogue, trainers, vendors, quest givers, shops, and spawned-NPC behavior beyond visibility.
- Social systems: party, friends, whispers, chat rooms, cross-channel social state.
- Guild systems: full guild creation, ranks, members, storage, guild war/wave behavior.
- Trade/private shop: full item/gold exchange validation, cancellation, persistence, client edge cases.
- Quests/tutorial/titles/fishing/digging: registered or skeletal pieces exist, but deeper gameplay remains.
- Pets/mounts/soulmate/mail/battleground: opcodes and stubs exist, but major behavior remains.

### Current Remaining TODO Clusters

- `cmd/gameserver/handlers_battleground.go`
- `cmd/gameserver/handlers_pets.go`
- `cmd/gameserver/handlers_soulmate.go`
- `cmd/gameserver/handlers_mail.go`
- `cmd/gameserver/handlers_chatroom.go`
- `cmd/gameserver/handlers_fishing.go`
- `cmd/gameserver/handlers_mounts.go`
- `cmd/gameserver/handlers_titles.go`
- `cmd/gameserver/handlers_character.go`
- `cmd/gameserver/handlers_combat.go`
- `cmd/gameserver/handlers_digging.go`
- `cmd/gameserver/handlers_monsters.go`
- `cmd/gameserver/handlers_resurrection.go`
- `cmd/gameserver/handlers_tutorial.go`

### Existing Test Coverage Signals

Agents should look for related tests before adding new behavior. Current test areas include:

- Packets, world data, item templates, inventory/categories, professions, NPC types.
- Movement, visibility, channels, weather, character stats, combat, monsters, items, item upgrade.
- NPC interactions, NPC vendors, trade runtime, social runtime, skills, professions, quest professions.

Use this map as a starting point, but always run the Go Gap Scanner before implementation because the active source can change quickly.

## Done Definition

A feature slice is done only when:

- The source map exists or has been summarized in the task.
- The Asda2 flow is understood.
- Packet layout is documented or covered by tests.
- Code is placed in the correct files.
- `gofmt -w .` has been run.
- `go test ./...` passes.
- `go vet ./...` passes or any failure is clearly explained.
- Remaining TODOs are specific and intentional.
