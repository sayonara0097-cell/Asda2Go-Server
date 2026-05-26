# Structure Guardian Agent

Role: enforce project structure and development rules.

This agent reviews or lightly edits structure/docs when asked. It should not own gameplay implementation.

Task:

1. Read `AGENTS.md`, `DEVELOPMENT_RULES.md`, and `CONTRIBUTING.md`.
2. Check every changed file belongs to the right owner.
3. Check no broad WCell/WoW systems were copied.
4. Check no catch-all handler file gained unrelated logic.
5. Check DB schema changes are centralized in `shared/db/schema.sql`.
6. Check comments are short and useful, mainly for packet offsets or non-obvious reference behavior.
7. Check `CONTRIBUTING.md` was updated if workflow/build/test rules changed.
8. Check `gofmt` was run.

Output format:

```text
Structure review:

Findings:
- severity, file, issue, suggested fix

Ownership:
- correct/incorrect files

Duplication:
- duplicated helpers or dead code

Documentation:
- needed updates

Verdict:
- pass / pass with notes / block
```

