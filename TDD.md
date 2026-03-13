# TDD Workflow Guide

Test-driven development workflow for peth. Every feature starts with a failing test.

## Quick Reference

```bash
make test              # run all tests
make test-v            # verbose output
make test-race         # with race detector
make test-cover        # run tests + print coverage %
make cover-html        # generate HTML coverage report
make cover-func        # per-function coverage breakdown
make cover-check       # fail if coverage < 80%
make test-run T=Name   # run a specific test by name
```

## The TDD Cycle

Every feature or bugfix follows Red → Green → Refactor:

### 1. Red — Write a Failing Test

Write the test **before** any implementation. The test defines the expected behavior.

```go
// internal/wallet/keystore_test.go
func TestKeystoreCreate(t *testing.T) {
    ks, err := NewKeystore(t.TempDir())
    require.NoError(t, err)

    addr, err := ks.Create("test-wallet")
    require.NoError(t, err)
    assert.True(t, common.IsHexAddress(addr))
}
```

Run it. It must fail:

```bash
make test-run T=TestKeystoreCreate
# FAIL — NewKeystore doesn't exist yet
```

### 2. Green — Write the Minimum Code to Pass

Implement only what the test requires. No extras.

```go
// internal/wallet/keystore.go
func NewKeystore(dir string) (*Keystore, error) {
    return &Keystore{dir: dir}, nil
}

func (ks *Keystore) Create(name string) (string, error) {
    key, err := crypto.GenerateKey()
    if err != nil {
        return "", err
    }
    addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
    // store key...
    return addr, nil
}
```

Run it again:

```bash
make test-run T=TestKeystoreCreate
# PASS
```

### 3. Refactor — Clean Up Without Changing Behavior

Improve the code. Tests must still pass after every change:

```bash
make test
```

---

## Project Structure for Tests

Tests live next to the code they test:

```
peth/
├── cmd/
│   └── peth/
│       ├── main.go
│       └── main_test.go
├── internal/
│   ├── client/
│   │   ├── client.go          # Pinchtab HTTP client
│   │   └── client_test.go
│   ├── wallet/
│   │   ├── keystore.go
│   │   ├── keystore_test.go
│   │   ├── provider.go        # mock EIP-1193 provider
│   │   └── provider_test.go
│   ├── chain/
│   │   ├── registry.go
│   │   └── registry_test.go
│   └── tx/
│       ├── builder.go
│       └── builder_test.go
└── testutil/                   # shared test helpers
    └── fixtures.go
```

## Test Categories

### Unit Tests (default)

Fast, isolated, no external dependencies. These are the bulk of your tests.

```go
func TestChainRegistryLookup(t *testing.T) {
    reg := chain.NewRegistry()
    c, err := reg.Get("optimism")
    require.NoError(t, err)
    assert.Equal(t, uint64(10), c.ChainID)
}
```

### Integration Tests (build tag)

Tests that need a running Pinchtab instance or local chain. Guarded by a build tag so they don't run in `make test`:

```go
//go:build integration

func TestPinchtabNav(t *testing.T) {
    client := client.New("http://localhost:9867")
    err := client.Nav("https://example.com")
    require.NoError(t, err)

    snap, err := client.Snap()
    require.NoError(t, err)
    assert.Contains(t, snap, "Example Domain")
}
```

Run integration tests explicitly:

```bash
go test -tags integration ./...
```

### Table-Driven Tests

Use for functions with multiple input/output cases:

```go
func TestParseChainID(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    uint64
        wantErr bool
    }{
        {"decimal", "10", 10, false},
        {"hex", "0xa", 10, false},
        {"name", "optimism", 10, false},
        {"unknown", "fakenet", 0, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseChainID(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

## Coverage

### Minimum Threshold: 80%

The project enforces an 80% coverage minimum:

```bash
make cover-check            # fails if below 80%
make cover-check COVER_MIN=90  # override threshold
```

### Checking Coverage Locally

```bash
# Quick summary
make test-cover
# total: (statements) 84.2%

# Per-function breakdown
make cover-func
# github.com/charkala/peth/internal/wallet/keystore.go:15:  Create    100.0%
# github.com/charkala/peth/internal/wallet/keystore.go:28:  Import     75.0%

# Visual HTML report (highlights uncovered lines)
make cover-html
open coverage.html
```

### What to Cover

| Must cover                                | Skip coverage for          |
|-------------------------------------------|----------------------------|
| All public functions                      | `main.go` entry point      |
| Error paths and edge cases                | Generated code             |
| Chain/wallet/tx business logic            | Thin CLI flag wiring       |
| Provider injection JS generation          | Third-party library calls  |

## Mocking External Dependencies

Use interfaces for anything that talks to Pinchtab or the network:

```go
// internal/client/client.go
type Pinchtab interface {
    Nav(url string) error
    Snap() (string, error)
    Eval(js string) (string, error)
    Click(ref string) error
}

// real implementation
type HTTPClient struct { baseURL string }

// test mock
type MockClient struct {
    SnapResult string
    EvalResult string
    EvalErr    error
}

func (m *MockClient) Eval(js string) (string, error) {
    return m.EvalResult, m.EvalErr
}
```

This keeps unit tests fast and deterministic without hitting a real Pinchtab instance.

## Running the Full Check Locally

Before pushing, run the same checks that would catch issues:

```bash
go vet ./...        # static analysis
make test-race      # tests with race detector
make cover-check    # coverage threshold
```

## TDD Checklist for New Features

Before starting any feature from the [ROADMAP](ROADMAP.md):

1. [ ] Write test(s) that describe the expected behavior
2. [ ] Run tests — confirm they **fail** (red)
3. [ ] Write the minimum implementation to pass
4. [ ] Run tests — confirm they **pass** (green)
5. [ ] Refactor if needed — tests must stay green
6. [ ] **Commit** — each passing Green or Refactor step is a good commit point
7. [ ] Run `make cover-func` — confirm new code is covered
8. [ ] Run `make cover-check` — confirm threshold is met
9. [ ] Run `make test-race` — confirm no data races
