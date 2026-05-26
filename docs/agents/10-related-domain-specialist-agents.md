# Related Domain Specialist Agents

Role: run only the domain agents related to the current feature.

These agents are inserted into the mandatory pipeline after the Packet/Protocol Agent and before the Go Porting Agent.

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

Do not run every domain specialist for every task. Pick only the related ones. If none are related, say:

```text
No related domain specialist needed for this feature.
```

## Output Format

```text
Related domain:
Why this agent is needed:
Relevant Go files:
Relevant C# reference files:
Implementation constraints:
Tests/runtime checks to require:
Risks:
```

## Foundation Domain Agents

### Packet/Opcode/Encryption Agent

Use for packet layout, opcode constants, packet reader/writer behavior, Asda2 XOR crypt, locale string encoding, router registration, or golden packet tests.

Likely Go areas:

- `shared/packet`
- `shared/crypt`
- `cmd/gameserver/router.go`
- `cmd/loginserver/router.go`

### Login/Character Flow Agent

Use for account auth, sessions, character list/create/select, handoff tokens, login-to-game redirect, duplicate sessions, and character initialization.

Likely Go areas:

- `cmd/loginserver`
- `cmd/gameserver/login.go`
- `cmd/gameserver/account_sessions.go`
- `shared/relay`
- `shared/db`

### World/Map/Movement/Visibility Agent

Use for world runtime, map registry, movement, position validation, WhoIsHere, map change, player visibility, and channel-bound world state.

Likely Go areas:

- `cmd/gameserver/world.go`
- `cmd/gameserver/world_entities.go`
- `cmd/gameserver/movement.go`
- `cmd/gameserver/handlers_movement.go`
- `cmd/gameserver/visibility.go`
- `cmd/gameserver/map_change.go`

### Static World Data/NPC/Monster/NpcServer Agent

Use for `data/asda2`, `shared/worlddata`, npcserver visibility, static NPC/monster templates, spawns, drops, movement paths, and npcserver HTTP sync.

Likely Go areas:

- `data/asda2`
- `shared/worlddata`
- `shared/npcruntime`
- `cmd/npcserver`
- `cmd/gameserver/npcs.go`
- `cmd/gameserver/monsters.go`
- `cmd/gameserver/npcserver_client.go`
- `cmd/gameserver/npcserver_visibility.go`

### Inventory Agent

Use for character inventory state, slots, bags, equipment slots, stack movement, item deletion, inventory packet behavior, and inventory persistence.

Likely Go areas:

- `shared/types/inventory.go`
- `shared/db/items.go`
- `cmd/gameserver/items.go`
- `cmd/gameserver/handlers_items.go`
- `cmd/gameserver/items_test.go`

### Item Template/Item Use Agent

Use for Asda2 item templates, item classification, item metadata, item usage, consumables, requirements, and compatibility fallbacks.

Likely Go areas:

- `shared/types/item_template.go`
- `shared/db/item_templates.go`
- `shared/db/item_template_store.go`
- `cmd/gameserver/item_use.go`
- `cmd/gameserver/handlers_items.go`

### Loot Agent

Use for monster drops, world drops, pickup flow, loot ownership, loot packets, and `cmd/gameserver/loot.go`.

Likely Go areas:

- `cmd/gameserver/loot.go`
- `cmd/gameserver/handlers_loot.go`
- `cmd/gameserver/monsters.go`
- `data/asda2/monsters/drops.json`

### Repair Items Agent

Use for durability repair, repair cost, repair packets, and item durability status.

Likely Go areas:

- `cmd/gameserver/handlers_repair.go`
- `cmd/gameserver/items.go`
- `shared/types/inventory.go`

### Shop Items Agent

Use for NPC shop/vendor buy/sell, shop item packets, shop pricing, and vendor stock.

Likely Go areas:

- `cmd/gameserver/handlers_shop_items.go`
- `cmd/gameserver/npc_vendors.go`
- `cmd/gameserver/handlers_npc_vendors.go`
- `shared/db/npc_vendors.go`

### Item Upgrade Agent

Use for upgrade/enchant/disassemble/crafting-related item upgrade behavior, item options, and upgrade result packets.

Likely Go areas:

- `cmd/gameserver/handlers_item_upgrade.go`
- `cmd/gameserver/item_options.go`
- `cmd/gameserver/item_upgrade_test.go`
- `shared/db/crafting.go`

### Character Stats Agent

Use for base stats, derived stats, HP/MP, stat packets, equipment stat application, and level/stat recalculation.

Likely Go areas:

- `cmd/gameserver/character_stats.go`
- `shared/stats`
- `shared/db/base_stats.go`
- `shared/types/base_stats.go`

### Profession/Skills Agent

Use for Warrior/Archer/Mage profession behavior, learned skills, skill templates, skill packets, profession restrictions, and profession progression.

Likely Go areas:

- `cmd/gameserver/professions.go`
- `cmd/gameserver/quest_professions.go`
- `cmd/gameserver/skills.go`
- `cmd/gameserver/skill_packets.go`
- `cmd/gameserver/skills_effects.go`
- `cmd/gameserver/handlers_skills.go`
- `shared/db/skills.go`

### Combat Agent

Use for player combat state, normal attacks, skill attacks, player health, damage packets, resurrection, and PvP hooks.

Likely Go areas:

- `cmd/gameserver/combat_normal.go`
- `cmd/gameserver/handlers_combat.go`
- `cmd/gameserver/player_health.go`
- `cmd/gameserver/handlers_resurrection.go`

### Monster Combat Agent

Use for monster aggro, monster attacks, death, respawn, monster movement during combat, loot generation, and NPC/monster runtime combat events.

Likely Go areas:

- `cmd/gameserver/monsters.go`
- `cmd/gameserver/handlers_monsters.go`
- `cmd/gameserver/loot.go`
- `cmd/npcserver`

### Teleport/Portals/Map-Change Agent

Use for teleport crystals, scrolls, bind locations, portals, map transitions, channel reconnect impact, and related packets.

Likely Go areas:

- `cmd/gameserver/handlers_teleport.go`
- `cmd/gameserver/portals.go`
- `cmd/gameserver/map_change.go`
- `shared/db/teleport.go`
- `shared/db/portals.go`

### Weather Agent

Use for in-game weather, client time sync, map weather state, and weather packets.

Likely Go areas:

- `cmd/gameserver/weather.go`
- `cmd/gameserver/handlers_weather.go`
- `shared/types/weather.go`
- `shared/db/weather.go`

### Trade/Private Shop Agent

Use for player trade, trade window, item/gold exchange, private shop open/buy/close, cancellation, and persistence.

Likely Go areas:

- `cmd/gameserver/trade_runtime.go`
- `cmd/gameserver/handlers_trade.go`
- `cmd/gameserver/private_shop_runtime.go`
- `cmd/gameserver/handlers_private_shop.go`

### Guild Agent

Use for guild creation, ranks, members, guild packets, guild DB, guild storage, guild war, and guild wave behavior.

Likely Go areas:

- `cmd/gameserver/guild_runtime.go`
- `cmd/gameserver/guild_logic.go`
- `cmd/gameserver/guild_packets.go`
- `cmd/gameserver/guild_handlers.go`
- `shared/db/guilds.go`

### Social/Chat Agent

Use for normal chat, whispers, party, friends, chat rooms, cross-channel social state, and social packet delivery.

Likely Go areas:

- `cmd/gameserver/social_runtime.go`
- `cmd/gameserver/social_packets.go`
- `cmd/gameserver/handlers_chat.go`
- `cmd/gameserver/handlers_friends.go`
- `cmd/gameserver/handlers_party.go`
- `cmd/gameserver/handlers_chatroom.go`
- `shared/db/social.go`

### NPC Interaction/Vendor/Trainer Agent

Use for NPC targeting, dialogue, vendors, trainers, quest givers, shops, and NPC interaction packets.

Likely Go areas:

- `cmd/gameserver/npc_interactions.go`
- `cmd/gameserver/npc_trainers.go`
- `cmd/gameserver/npc_vendors.go`
- `cmd/gameserver/handlers_npc_dialogue.go`
- `cmd/gameserver/handlers_npc_trainers.go`
- `cmd/gameserver/handlers_npc_vendors.go`
- `shared/db/npc_vendors.go`

### Relay/GM Tool Agent

Use for relay contracts, game-server registry, channel status, handoffs, world announcements, GM API, GM permissions, and gmtool behavior.

Likely Go areas:

- `shared/relay`
- `cmd/relayserver`
- `cmd/gmtool`
- `cmd/gameserver/relay.go`
- `cmd/gameserver/gm.go`
- `cmd/gameserver/gm_*.go`

## Depth/Advanced Domain Agents

### Inventory Depth Agent

Use for equipment stats, requirements, durability, all stack/equip/use/delete edge cases, full Asda2 item formulas, and item-loss/duplication risk.

### Skills Depth Agent

Use for complete effects, buffs/debuffs, cooldowns, cast/range/resource validation, SoulGuard skills, and Soulmate skills.

### Combat Depth Agent

Use for full damage/defense formulas, PvP, advanced status effects, real-client combat packet validation, and combat regression risks.

### NPC Systems Depth Agent

Use for dialogue, trainers, vendors, quest givers, shops, spawned-NPC behavior beyond visibility, and NPC ownership boundaries.

### Social Systems Depth Agent

Use for party, friends, whispers, chat rooms, cross-channel social state, relay-backed online status, and social persistence.

### Guild Systems Depth Agent

Use for full guild creation, ranks, members, guild storage, guild permissions, guild war, and guild wave.

### Trade/Private Shop Depth Agent

Use for full item/gold exchange validation, cancellation, rollback, persistence, private shop client edge cases, and duplication/loss prevention.

### Quests/Tutorial/Titles/Fishing/Digging Agent

Use for quest handlers, tutorial flow, title discovery/getting, fishing, digging, and related reward/progression packets.

Likely Go areas:

- `cmd/gameserver/handlers_npc_quests.go`
- `cmd/gameserver/handlers_tutorial.go`
- `cmd/gameserver/handlers_titles.go`
- `cmd/gameserver/handlers_fishing.go`
- `cmd/gameserver/handlers_digging.go`

### Pets/Mounts/Soulmate/Mail/Battleground Agent

Use for pet lifecycle, mounts, soulmate relationship/skills/messages, mail mailbox/send/read/delete, battleground registration/war/PvP systems, and existing opcode stubs.

Likely Go areas:

- `cmd/gameserver/handlers_pets.go`
- `cmd/gameserver/handlers_mounts.go`
- `cmd/gameserver/handlers_soulmate.go`
- `cmd/gameserver/handlers_mail.go`
- `cmd/gameserver/handlers_battleground.go`

