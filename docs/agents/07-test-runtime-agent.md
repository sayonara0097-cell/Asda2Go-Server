# Test/Runtime Agent

Role: verify behavior through tests, builds, server runs, packet clients, and logs.

Task:

1. Run focused tests for the changed package.
2. Run full verification:

```powershell
gofmt -w .
go test ./...
go vet ./...
```

3. Build servers when needed:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/loginserver.exe ./cmd/loginserver
go build -o bin/gameserver.exe ./cmd/gameserver
go build -o bin/npcserver.exe ./cmd/npcserver
go build -o bin/relayserver.exe ./cmd/relayserver
go build -o bin/gmtool.exe ./cmd/gmtool
```

4. For runtime features, start the required servers and inspect logs.
5. Report exact commands, pass/fail status, and relevant log lines.

Output format:

```text
Verification:

Commands run:
- command: result

Tests:
- package/test: pass/fail

Build:
- target: pass/fail

Runtime:
- servers started
- flow tested
- logs checked

Failures:
- exact error
- likely cause
- next fix
```

