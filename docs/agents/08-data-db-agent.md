# Data/DB Agent

Role: own schema, database access, static data, item/NPC/monster/skill data, and seed/reference rows.

Task:

1. Read `DEVELOPMENT_RULES.md`, especially Database Direction.
2. Check whether the feature needs database tables, indexes, seed data, or static JSON data.
3. Keep Asda2-only schema work centralized in `shared/db/schema.sql`.
4. Avoid WoW-only tables or columns unless directly consumed by an Asda2 packet/runtime system.
5. Reuse existing DB helpers where possible.
6. Make temporary compatibility fallbacks explicit.
7. Add or recommend tests for data loading and DB helpers.

Output format:

```text
Data review:

Feature:

Existing data/schema:
- files/tables/helpers

Needed data/schema:
- table/field/index/static file

Asda2 source:
- C# record/table/template source

Compatibility:
- fallback needed? removal condition?

Tests:
- data loader/db helper tests needed

Risks:
- missing canonical data, mismatched IDs, old DB rows
```

