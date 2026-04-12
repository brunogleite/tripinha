---
name: tdd
description: >
  Go TDD pairing mode. Enforces red-green-refactor strictly, one cycle at a time.
  Writes tests first, minimum code to pass, then refactors. Flags anti-patterns inline.
  Inspired by quii.gitbook.io/learn-go-with-tests. Use when user says "TDD this",
  "write tests first", "do TDD", or invokes /tdd.
---

# TDD — Go

Strict red-green-refactor. No implementation before a failing test. No skipping steps.

Source: [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests)

---

## The Cycle — follow this exactly, every time

```
1. RED    — write a failing test. Run it. Confirm it fails with a useful message.
2. GREEN  — write the minimum code to make it pass. No more.
3. REFACTOR — clean up. Tests must stay green. Behavior must not change.
```

Repeat. One small cycle at a time. Don't bundle multiple behaviors into one cycle.

---

## Writing the Test (RED)

**File:** `foo_test.go` (same package, or `foo_test` package for black-box testing)

**Function signature:**
```go
func TestFunctionName(t *testing.T) {}
```

**Subtests** — use for distinct scenarios:
```go
func TestArea(t *testing.T) {
    t.Run("rectangles", func(t *testing.T) { ... })
    t.Run("circles",    func(t *testing.T) { ... })
}
```

**Table-driven tests** — use when same behavior, multiple inputs:
```go
func TestArea(t *testing.T) {
    tests := []struct {
        name  string
        shape Shape
        want  float64
    }{
        {name: "rectangle", shape: Rectangle{Width: 12, Height: 6}, want: 72.0},
        {name: "circle",    shape: Circle{Radius: 10},              want: 314.1},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.shape.Area()
            if got != tt.want {
                t.Errorf("%s: got %.2f, want %.2f", tt.name, got, tt.want)
            }
        })
    }
}
```

**Error messages must be meaningful:**
```go
// Bad
t.Errorf("wrong result")

// Good
t.Errorf("got %q, want %q", got, want)

// For structs — use %#v to print field values
t.Errorf("got %#v, want %#v", got, want)
```

**Test helpers** — always call `t.Helper()` so failures report at the call site:
```go
func assertArea(t testing.TB, got, want float64) {
    t.Helper()
    if got != want {
        t.Errorf("got %.2f, want %.2f", got, want)
    }
}
```

Use `testing.TB` (not `*testing.T`) in helpers — it works for both tests and benchmarks.

---

## Writing the Implementation (GREEN)

- Write **only** what makes the test pass. Nothing more.
- If the compiler fails first, fix the compiler. That is a valid GREEN step.
- Hardcoding a return value to pass the first test is fine — the next test will force real logic.
- Do not anticipate future tests. Do not add behavior not yet covered by a failing test.

---

## Refactoring

- You can change **anything** while tests are green — names, structure, abstractions.
- You cannot change behavior. Tests must pass unchanged.
- Refactoring is not optional. Clean code after every green, before moving to the next test.
- Extract helpers, rename, simplify — but only under green.

---

## Interface Design

Design interfaces small and late — let tests drive when an interface is needed:

```go
// Don't define a large interface upfront.
// Define the smallest interface your function actually needs:
type Storer interface {
    Save(key string, value []byte) error
}
// Not:
type Storer interface {
    Save(...) error
    Delete(...) error
    List(...) ([]string, error)
    // ... 10 more methods the function doesn't use
}
```

**Rule:** Expose a concrete struct. Let callers define the small interface they need.
Go interfaces are satisfied implicitly — no `implements` keyword needed.

**For I/O dependencies** — inject `io.Writer` / `io.Reader`, not concrete types:
```go
// Testable: inject the writer
func Greet(w io.Writer, name string) {
    fmt.Fprintf(w, "Hello, %s", name)
}

// In tests: inject bytes.Buffer
buf := &bytes.Buffer{}
Greet(buf, "Bruno")

// In prod: inject os.Stdout or http.ResponseWriter
Greet(os.Stdout, "Bruno")
```

---

## Mocking

Use mocks when: external service, DB, time, randomness, or any dependency that can fail or slow tests.

**Spy pattern** — implement the interface, record calls:
```go
type SpyStore struct {
    getCalls  []string
    saveCalls []string
}

func (s *SpyStore) Get(key string) string {
    s.getCalls = append(s.getCalls, key)
    return ""
}
```

**Max 3 mocks per test.** More than 3 = design problem, not a testing problem. Rethink.

**Don't mock what you don't own.** Wrap third-party clients behind your own interface, mock the wrapper.

---

## Anti-patterns — flag these inline when spotted

| Anti-pattern | What it looks like | Fix |
|---|---|---|
| Evergreen test | Test cannot fail; written after code | Delete and rewrite test-first |
| Useless assertion | `"false was not equal to true"` | Add message: `got X, want Y` |
| Asserting irrelevant detail | Comparing entire struct when only one field matters | Assert only the field you care about |
| Too many assertions | 10 `if got != want` in one test | Split into subtests |
| Excessive setup | 100+ lines before the assertion | Code does too much; split responsibilities |
| Too many mocks | >3 mocks in one test | Redesign the abstraction |
| Public function for test access | `func getInternalState()` exported only for tests | Use `_test` package or test public behavior |
| Complicated table test | Table has bool columns like `wantError`, unrelated scenarios mixed | Break into separate `TestX_ErrorCase` |
| Leaky interface | Interface has 8+ methods | Shrink interface; expose struct |
| Test mode in prod code | `if testing { ... }` | Inject the dependency instead |
| Testing private functions | `TestcomputeInternalHash` | Test public behavior that uses it |

---

## Benchmarks

Add benchmarks alongside tests when performance matters:

```go
func BenchmarkRepeat(b *testing.B) {
    for b.Loop() {
        Repeat("a", 5)
    }
}
```

Run: `go test -bench=. -benchmem`

---

## Workflow in this skill

When the user asks to implement something:

1. **Ask what behavior to test first** if not obvious. One behavior per cycle.
2. **Write the test.** Show it. Confirm it's RED (compiler error or failing assertion).
3. **Write minimum implementation.** Show it. Confirm it's GREEN.
4. **Refactor.** Show the cleaned version. Confirm tests still pass.
5. **Ask:** "Next behavior?" — repeat from step 1.

If at any point a test is hard to write, **stop and redesign** before writing implementation.
Hard-to-test = bad design. Don't work around it; fix the design.

---

## Boundaries

- Never write implementation before a failing test exists. No exceptions.
- "stop tdd" or "normal mode": exit TDD mode, revert to regular assistant behavior.
- If the user asks to add code without a test: write the test first, then ask them to confirm RED before proceeding.
