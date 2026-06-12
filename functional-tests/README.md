# Functional Tests for `rhc`

> Functional testing most closely aligns with the concept of user experience, and it focuses on one critical question: **Do software features work as expected and in accordance with specified requirements?** ([IBM](https://www.ibm.com/think/topics/functional-testing))

BDD-style functional tests using [Godog](https://github.com/cucumber/godog) (Gherkin syntax).

The test suite is built as a standalone binary (`rhc-functional`) from
`cmd/functional/`. Feature files and fixture configuration live here in
`functional-tests/`.

## Required Environment Variables

- `TARGET`: One of `hosted`, `satellite`, `local`.
- `CONF`: Pre-populated directory with program configurations.
- `TMT_PLAN_DATA`: Place for `artifacts` directory (defaults to `.`).

## Running

Build the binary once:

```bash
make build-functional
```

Then run it from this directory so the relative `features/` path resolves:

```bash
cd functional-tests
TARGET=local CONF=./conf-local ../rhc-functional
```

Pass godog flags directly on the command line:

```bash
# Satellite-specific tests only
TARGET=satellite CONF=/tmp/conf-sat ../rhc-functional --godog.tags="@only-satellite || @tier1"

# Validate a specific feature directory
TARGET=hosted CONF=/tmp/conf-hosted ../rhc-functional --godog.paths="features/configure/"
```

## Conventions

- Tiers: `@tier1`, `@tier2`, `@tier3`
- Speed: `@fast`, `@slow`
- Target: `@only-local`, `@only-hosted`, `@only-satellite`
- Other: `@wip`
