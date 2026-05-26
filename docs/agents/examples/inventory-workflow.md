# Inventory Agent Workflow Example

Use this example to test the agent system on inventory work.

Feature:

```text
Inventory item move / equip / use / split stack / delete
```

Reference source:

```text
<reference-source-root>
```

Active Go source:

```text
<active-go-source-root>
```

## Mandatory Agent Order

```text
Reference Mapper
-> Go Gap Scanner
-> Flow Analyst
-> Packet/Protocol Agent
-> Related Domain Specialist Agent(s)
-> Go Porting Agent
-> Structure Guardian
-> Test/Runtime Agent
-> Data/DB Agent
-> Regression Reviewer
```

Inventory usually touches item templates, item rows, character inventory state, packet layouts, and DB persistence, so include the Inventory Agent, often the Inventory Depth Agent, and the Data/DB Agent.

## 1. Reference Mapper

Prompt:

```text
Use docs/agents/01-reference-mapper.md.

Feature: Inventory item move / equip / use / split stack / delete

Map the C# reference inventory system. Focus on:
- WCell.RealmServer\Handlers\Asda2InventoryHandler.cs
- Asda2 item enums/status codes
- packet handlers
- outgoing packet writers
- item movement/equip/use/delete logic
- inventory DB records or item persistence

Do not edit Go code.
```

Expected output:

- C# files and methods.
- Packet handlers and packet writers.
- Status codes and enum values.
- State touched by inventory operations.
- WCell/WoW logic to ignore.

## 2. Go Gap Scanner

Prompt:

```text
Use docs/agents/02-go-gap-scanner.md.

Feature: Inventory item move / equip / use / split stack / delete

Scan the active Go source for existing inventory code. Focus on:
- cmd/gameserver/handlers_items.go
- cmd/gameserver/items.go
- cmd/gameserver/item_use.go
- cmd/gameserver/handlers_item_upgrade.go
- shared/types/inventory.go
- shared/types/item_template.go
- shared/db/items.go
- shared/db/item_templates.go
- shared/db/schema.sql
- tests related to items/inventory

Do not edit code. Recommend the smallest safe slice.
```

Expected output:

- Existing Go coverage.
- Missing handlers/TODOs.
- Correct edit files.
- Tests already available.
- Smallest next implementation slice.

## 3. Flow Analyst

Prompt:

```text
Use docs/agents/03-flow-analyst.md.

Feature: Inventory item move / equip / use / split stack / delete

Using the Reference Mapper and Go Gap Scanner outputs, describe the exact Asda2 inventory flow:
- request fields
- validation
- inventory state change
- DB persistence
- response packets
- broadcasts, if any
- WCell/WoW behavior to ignore
- recommended implementation slices

Do not edit code.
```

Expected output:

- Clear behavior spec.
- Small slices, for example:
  1. move item inside regular inventory
  2. equip/unequip item
  3. split stack
  4. delete item
  5. use consumable

## 4. Packet/Protocol Agent

Prompt:

```text
Use docs/agents/04-packet-protocol-agent.md.

Feature: Inventory item move / equip / use / split stack / delete

Verify packet compatibility:
- opcodes in shared/packet/opcodes.go
- request read order from C#
- response write order from C#
- Go packet writers/readers
- packet tests needed

Do not implement gameplay logic.
```

Expected output:

- Opcode alignment.
- Request/response field order.
- Packet tests to add.
- Unknown offsets or risks.

## 5. Go Porting Agent

Before this step, run the related domain specialist prompt:

```text
Use docs/agents/10-related-domain-specialist-agents.md.

Feature: Inventory item move / equip / use / split stack / delete

Run only the related domain agents:
- Inventory Agent
- Inventory Depth Agent, if equipment stats, requirements, durability, stack/equip/use/delete edge cases, item formulas, or item-loss/duplication risk are involved
- Item Template/Item Use Agent, if item metadata or item use is involved

Do not edit code. Produce implementation constraints, required tests, and risks.
```

Prompt:

```text
Use docs/agents/05-go-porting-agent.md.

Feature slice: <choose one small slice from the Flow Analyst output>

Implement only this slice in the active Go source.
Follow DEVELOPMENT_RULES.md and CONTRIBUTING.md.
Preserve packet field order.
Reuse existing helpers.
Add focused tests.
Run gofmt -w .
```

Recommended first slice:

```text
Move item inside regular inventory without equip/use behavior.
```

## 6. Structure Guardian

Prompt:

```text
Use docs/agents/06-structure-guardian.md.

Review the inventory implementation for:
- correct file ownership
- no unrelated WCell/WoW logic
- no duplicated helpers
- schema placement
- comments and naming
- documentation updates
```

## 7. Test/Runtime Agent

Prompt:

```text
Use docs/agents/07-test-runtime-agent.md.

Run verification for the inventory slice:
- gofmt -w .
- go test ./...
- go vet ./...

If runtime verification is useful, build the servers and test the packet flow.
Report exact pass/fail output.
```

## 8. Data/DB Agent

Prompt:

```text
Use docs/agents/08-data-db-agent.md.

Review whether this inventory slice needs DB/schema/static data changes:
- shared/db/schema.sql
- shared/db/items.go
- shared/db/item_templates.go
- shared/types/inventory.go
- shared/types/item_template.go

If no data/schema change is needed, say so explicitly.
```

## 9. Regression Reviewer

Prompt:

```text
Use docs/agents/09-regression-reviewer.md.

Review the completed inventory slice. Focus on:
- packet mismatches
- item duplication/loss bugs
- DB persistence bugs
- missing tests
- ownership violations
- unrelated changes

Give approve / approve with notes / request changes.
```
