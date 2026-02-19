# Code Review — go_api_nosql

Full review covering security, Go idioms ([CLAUDE.md](./CLAUDE.md)), bugs, coding practices, and style.

---

## 🟡 Go Patterns (CLAUDE.md violations)

### PAT-2: Service interfaces are too large (Rule 4.3)

> *"Interfaces with more than three methods are a red flag and should be re-evaluated."*

| Interface | Methods |
|---|---|
| `user.Service` | 6 |
| `file.Service` | 5 |
| `session.Service` | 4 |

Note: `auth.Service` has already been split into `PasswordRecoveryService`, `EmailConfirmationService`, and `PhoneConfirmationService`.

**Fix:** Apply the same pattern to the remaining services. For example, split `user.Service` into a reader and a writer, or group by caller (admin vs. self-service).

> **Not required ATM.** This is a testability concern only — no functional impact. The current interface sizes are acceptable for the scale of this project. Revisit when adding unit tests for handlers, at which point mocking smaller interfaces becomes valuable.

---

### PAT-4: `Deps` struct couples transport directly to infrastructure (Rule 4.2)
**File:** `internal/transport/http/router.go:31-44`

`Deps` imports and holds `*dynamo.*` concrete types. The transport layer should depend on interfaces, not concrete implementations.

> **Not required ATM.** `Deps` lives at the application's composition root where wiring concrete types is standard Go practice. The impact is limited to router-level integration tests, which don't exist yet. If a test harness for `NewRouter` is ever added, this should be revisited then.

---

## Summary

| Category | Count | Critical/High |
|----------|-------|---------------|
| 🟡 Go Patterns | 2 | 0 |
| **Total** | **2** | **0** |

