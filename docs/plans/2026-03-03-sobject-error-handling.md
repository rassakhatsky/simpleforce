# SObject CRUD Error Handling Redesign

## Overview
- Change SObject CRUD methods (Get, Create, Update, Upsert, Delete) to return explicit errors instead of silently returning nil
- Aligns the library with Go conventions: callers get actionable error information instead of ambiguous nil returns
- Breaking change — method signatures change from `*SObject` to `(*SObject, error)`, which removes method chaining on CRUD operations
- Also fixes the Delete bug where the `id` parameter is accepted but ignored (line 262 uses `obj.ID()` instead of `oid`)

## Context (from discovery)
- Files/components involved: `sobject.go` (primary), `sobject_test.go`, `errorHelpers.go` (sentinel errors)
- Related patterns found: `force.go` already follows `(value, error)` pattern (Query, ApexREST, LoginPassword)
- Dependencies identified: integration tests heavily chain CRUD calls and will need significant updates
- Current logging uses `log.Println()` directly in sobject.go instead of `client.logger` — will fix as part of this refactoring since error messages are being reworked

## Development Approach
- **Testing approach**: TDD — write/update tests for new signatures first, then change implementation
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
  - tests are not optional — they are a required part of the checklist
  - write unit tests for new functions/methods
  - write unit tests for modified functions/methods
  - add new test cases for new code paths
  - update existing test cases if behavior changes
  - tests cover both success and error scenarios
- **CRITICAL: all tests must pass before starting next task** — no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- Run tests after each change
- Maintain backward compatibility within the new API (no further breaking changes after this)

## Testing Strategy
- **Unit tests**: required for every task (see Development Approach above)
- Unit tests that don't need SF credentials are the primary verification mechanism
- Integration tests (require `SF_USER`, `SF_PASS`, etc.) will be updated but won't run in CI without credentials

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if implementation deviates from original scope
- Keep plan in sync with actual work done

## What Goes Where
- **Implementation Steps** (`[ ]` checkboxes): tasks achievable within this codebase — code changes, tests, documentation updates
- **Post-Completion** (no checkboxes): items requiring external action — manual testing with SF credentials, consuming project updates

## Implementation Steps

### Task 1: Add sentinel errors for SObject validation failures
- [x] Define new sentinel errors in `errorHelpers.go`: `ErrObjectTypeMissing`, `ErrObjectClientMissing`, `ErrObjectIDMissing`, `ErrExternalIDMissing`, `ErrMarshalRequest`, `ErrHTTPRequest`, `ErrParseResponse`
- [x] Write unit tests for new error variables (verify they are distinct, implement `error` interface)
- [x] Run tests — must pass before task 2

### Task 2: Change Get to return (*SObject, error)
- [x] Write unit tests for `GetWithContext` expecting `(*SObject, error)` return: test missing type, missing client, missing ID, successful get (using mock/test SObject)
- [x] Update `GetWithContext` signature to return `(*SObject, error)` — return wrapped sentinel errors with context
- [x] Update `Get` wrapper to return `(*SObject, error)` and delegate to `GetWithContext`
- [x] Replace `log.Println()` calls in Get with `client.logger.Println()` where client is available, remove where returning error
- [x] Run tests — must pass before task 3

### Task 3: Change Create to return (*SObject, error)
- [x] Write unit tests for `CreateWithContext` expecting `(*SObject, error)` return: test missing type, missing client, marshal failure, HTTP failure, parse failure
- [x] Update `CreateWithContext` signature to return `(*SObject, error)` — return wrapped sentinel errors
- [x] Update `Create` wrapper to return `(*SObject, error)`
- [x] Replace `log.Println()` calls in Create with `client.logger.Println()` / error returns
- [x] Run tests — must pass before task 4

### Task 4: Change Update to return (*SObject, error)
- [x] Write unit tests for `UpdateWithContext` expecting `(*SObject, error)` return: test missing type, missing client, missing ID, marshal failure, HTTP failure
- [x] Update `UpdateWithContext` signature to return `(*SObject, error)`
- [x] Update `Update` wrapper to return `(*SObject, error)`
- [x] Replace `log.Println()` calls in Update with `client.logger.Println()` / error returns
- [x] Remove orphaned `log.Println(string(respData))` on line 188
- [x] Run tests — must pass before task 5

### Task 5: Change Upsert to return (*SObject, error)
- [x] Write unit tests for `UpsertWithContext` expecting `(*SObject, error)` return: test missing type, missing client, missing external ID field, missing external ID, marshal failure, HTTP failure, parse failure
- [x] Update `UpsertWithContext` signature to return `(*SObject, error)`
- [x] Update `Upsert` wrapper to return `(*SObject, error)`
- [x] Replace `log.Println()` calls in Upsert with `client.logger.Println()` / error returns
- [x] Remove debug logging of ExternalID/ExternalIDField on lines 200-201
- [x] Run tests — must pass before task 6

### Task 6: Fix Delete bug and clean up logging
- [ ] Write unit test for `DeleteWithContext` with explicit `id` parameter — verify it uses the provided ID, not `obj.ID()`
- [ ] Fix line 262: change `obj.ID()` to `oid` so the `id` parameter is actually used in the URL
- [ ] Replace `log.Println(url)` on line 263 with `client.logger.Println()`
- [ ] Replace `log.Println()` usage with `client.logger.Println()` where client is available
- [ ] Run tests — must pass before task 7

### Task 7: Update integration tests for new signatures
- [ ] Update `TestSObject_Get` — handle `(*SObject, error)` returns, verify errors on negative cases
- [ ] Update `TestSObject_Create` — handle `(*SObject, error)` returns, verify errors on negative cases, remove chaining
- [ ] Update `TestSObject_Update` — break chain into explicit steps with error checks
- [ ] Update `TestSObject_Upsert` — handle `(*SObject, error)` returns, verify errors on negative cases
- [ ] Update `TestSObject_Delete` — update to use new Create/Get signatures
- [ ] Update `TestSObject_GetUpdate` — break chain into explicit steps with error checks
- [ ] Run tests — must pass before task 8

### Task 8: Verify acceptance criteria
- [ ] Verify all CRUD methods return `(*SObject, error)` or `error` (Delete)
- [ ] Verify Delete uses `oid` not `obj.ID()` when id parameter is provided
- [ ] Verify no `log.Println()` calls remain in sobject.go (all use `client.logger`)
- [ ] Verify orphaned logging (line 188) and debug logging (lines 200-201) are removed
- [ ] Run full test suite (`go test ./...`)
- [ ] Run linter (`go vet ./...`)
- [ ] Verify all unit tests pass without SF credentials

### Task 9: [Final] Update documentation
- [ ] Update CLAUDE.md — update "Fluent API" section to reflect that CRUD methods now return `(*SObject, error)` instead of `*SObject`
- [ ] Update CLAUDE.md — note that `log.Println()` in sobject.go has been replaced with `client.logger`
- [ ] Add/update code comments on changed method signatures

## Technical Details

### Signature Changes
```
Get(id ...string) *SObject                              → Get(id ...string) (*SObject, error)
GetWithContext(ctx, id ...string) *SObject               → GetWithContext(ctx, id ...string) (*SObject, error)
Create() *SObject                                        → Create() (*SObject, error)
CreateWithContext(ctx) *SObject                           → CreateWithContext(ctx) (*SObject, error)
Update() *SObject                                        → Update() (*SObject, error)
UpdateWithContext(ctx) *SObject                           → UpdateWithContext(ctx) (*SObject, error)
Upsert() *SObject                                        → Upsert() (*SObject, error)
UpsertWithContext(ctx) *SObject                           → UpsertWithContext(ctx) (*SObject, error)
Delete(id ...string) error                               → Delete(id ...string) error (unchanged, but bug fix)
DeleteWithContext(ctx, id ...string) error                → DeleteWithContext(ctx, id ...string) error (unchanged, but bug fix)
```

### Error Wrapping Strategy
Each method returns a specific sentinel error wrapped with context:
```go
// Example: GetWithContext
if obj.Type() == "" {
    return nil, fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
}
if obj.client() == nil {
    return nil, fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
}
```

Callers can use `errors.Is()` to check for specific failure categories:
```go
obj, err := client.SObject("Case").Get(id)
if errors.Is(err, simpleforce.ErrObjectIDMissing) {
    // handle missing ID
}
```

### Unchanged Methods
- `Set(key, value) *SObject` — still returns `*SObject` for chaining (no I/O, can't fail)
- `Describe() *SObjectMeta` — out of scope (separate refactoring if desired)
- `SObjectField()`, `StringField()`, `TimeField()` — read-only accessors, unchanged

## Post-Completion

**Manual verification** (if applicable):
- Run integration tests with SF credentials to verify CRUD operations still work end-to-end
- Test against a Salesforce sandbox (not production) since tests create/delete Cases

**External system updates** (if applicable):
- Any consuming projects will need to update their code to handle the new `(value, error)` return signatures
- Consider a CHANGELOG entry or version bump (major version since this is a breaking change)
