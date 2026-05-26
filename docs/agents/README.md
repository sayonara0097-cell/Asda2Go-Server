# AsdaGo Agent Prompts

These prompt files are designed for working from VS Code while porting the Asda2 C# reference into the Go rewrite.

## How To Use

Open this folder in VS Code:

```text
<active-go-source-root>
```

When starting a feature, copy the relevant prompt into your agent/chat tool and fill the feature name, for example:

```text
Feature: Mail delete/read flow
Reference source: <reference-source-root>
Active Go source: <active-go-source-root>
```

Use the prompts in this mandatory order:

```text
00-shared-context.md
01-reference-mapper.md
02-go-gap-scanner.md
03-flow-analyst.md
04-packet-protocol-agent.md
10-related-domain-specialist-agents.md, if related
05-go-porting-agent.md
06-structure-guardian.md
07-test-runtime-agent.md
08-data-db-agent.md, only when schema/data is involved
09-regression-reviewer.md
```

## Main Rule

Use every agent, but only one implementation agent should edit code for a feature at a time. The other agents should produce maps, specs, reviews, and verification notes.

The required feature pipeline is always:

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

For systems that probably touch database or static data, such as inventory, skills, items, NPCs, monsters, quests, crafting, guilds, and mail, include the Data/DB Agent. If no data/schema changes are needed, the Data/DB Agent should say that explicitly.

For system-specific work, run `10-related-domain-specialist-agents.md` after the Packet/Protocol Agent and before the Go Porting Agent. Pick only the related domain agents. For example, inventory work uses Inventory Agent and often Inventory Depth Agent; guild work uses Guild Agent and maybe Guild Systems Depth Agent.

## Example

For an inventory feature, start with:

```text
Feature: Inventory item move / equip / use / split stack / delete
Reference source: <reference-source-root>
Active Go source: <active-go-source-root>
```

Then run each prompt in the mandatory order above.
