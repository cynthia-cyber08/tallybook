<!-- GENERATED FILE - DO NOT EDIT BY HAND.
     Source of truth: docs/WAVE3_BACKLOG.json
     Regenerate with: python3 scripts/gen-wave-backlog.py  (or `make backlog`)
     Any hand edit here is lost on the next regeneration. -->

# Wave 3 backlog

**145 issues · 25000 points.**

| Tier | Label | Count | Points each | Subtotal |
|---|---|---:|---:|---:|
| high | `complexity: high` | 1 | 340 | 340 |
| medium | `complexity: medium` | 10 | 255 | 2550 |
| low | `complexity: low` | 134 | 165 | 22110 |
| **total** | | **145** | | **25000** |

Complexity uses this repository's existing `complexity: low|medium|high` labels (see `gh label list`); `trivial` in the wave brief maps to `low`. Grades describe real scope, per `MAINTAINER.md` section 2 — they are not adjusted to reach a point total.

`area` is taken from the commit scopes `CONTRIBUTING.md` already defines, since the repository has no `area:` labels. It appears in each issue body rather than as a label.

---

## 1. test(settle): raise integration coverage of the settler above 60 percent

`complexity: high` · 340 points · area `settle`

**What's wrong.** `go test ./internal/... -cover` reports **8.4% for internal/settle**, the lowest of any package, against 1248 lines of source. CLAUDE.md §1 is explicit that this is "the component that decides whether the operator gets paid" and §6 that every decision in it favours durability. Much of the gap is that its tests need Postgres and skip without it, but that means the money path is effectively unexercised on any machine without Docker, and coverage in CI is the only place it is ever measured. A regression in Decide, Submit or ReconcileSubmitting would not be caught locally by anyone.

**Proposed fix.** The settler has the lowest measured coverage in the repository. Bring its Postgres-backed paths under test: exposure calculation, the four sweep triggers, submission intent and restart reconciliation, retry escalation, and the metrics queries.

**Acceptance criteria**

1. `go test ./internal/settle/ -cover` reports at least 60% with Postgres available
2. Each of §6's four sweep triggers has a test, plus the wait case and the never-settle-at-or-below-last-settled rule
3. ReconcileSubmitting is covered for both outcomes: covered by live withdrawn, and not covered
4. A coverage floor is asserted in CI so the number cannot silently regress
5. No test is added that passes without Postgres by skipping; scripts/test-ci.sh already fails on skips

**Guidance**

- internal/settle/helpers_test.go holds the existing Postgres guard — reuse it rather than inventing a second pattern.
- internal/settle/policy.go's Decide is a pure function and needs no database; the cheapest coverage is there first.
- Measure before and after with -coverprofile and put both numbers in the PR, so the claim is checkable.
- Do not chase the number with tests that assert nothing; §6's ordering rules are the behaviours worth pinning.

---

## 2. test(convert): add the missing test file for NumericToBigInt

`complexity: medium` · 255 points · area `convert`

**What's wrong.** `internal/convert` contains exactly one file, numeric.go, and **no test file whatsoever** — `go test -cover` reports 0.0%. NumericToBigInt at internal/convert/numeric.go:23 is the single conversion between Postgres NUMERIC(39,0) and the *big.Int every amount in this system is carried as, used by settle, meter and indexer alike. Its own doc comment records a bug found the hard way — a positive Exp from a real SUM() result — and nothing pins that behaviour. CLAUDE.md §8: a rounding bug here is a financial bug.

**Proposed fix.** internal/convert has no test file at all. Add one covering the exponent handling, the NULL and nil-Int cases, and the fractional-exponent rejection.

**Acceptance criteria**

1. internal/convert/numeric_test.go exists and `go test ./internal/convert/ -cover` reports at least 90%
2. A table-driven test covers Exp>0 scaling, Exp==0, Exp<0 rejection, Valid==false and Int==nil
3. The Exp>0 case asserts an exact *big.Int value, not just absence of error
4. The fractional-exponent case asserts the error rather than a truncated amount

**Guidance**

- The doc comment at numeric.go:13-21 records why Exp>0 must be handled — the test should pin that case specifically.
- pgtype.Numeric can be constructed directly in a test; no database is needed, so this runs everywhere.
- Assert with big.Int.Cmp, never by comparing String() output.

---

## 3. test(indexer): raise measured coverage of internal/indexer

`complexity: medium` · 255 points · area `indexer`

**What's wrong.** `go test ./internal/... -cover` reports **12.8% for internal/indexer** against 874 lines. CLAUDE.md §8 requires "every error path tested, not just happy paths"; at this level most of them are not. Part of the gap is that the Postgres-backed tests skip without Docker, which means the real number is only ever observed in CI and nobody sees a regression locally.

**Proposed fix.** `internal/indexer` measures 12.8% across 874 lines of source. Bring its ingestion, cursor persistence and reconciliation under test, concentrating on the error paths rather than the happy one.

**Acceptance criteria**

1. `go test ./internal/indexer/ -cover` improves by at least 20 percentage points with Postgres available
2. Every exported function in the package has at least one test reaching its error path
3. Before and after coverage numbers are recorded in the PR description
4. No test passes by skipping; scripts/test-ci.sh fails the build on a skip

**Guidance**

- internal/indexer's existing tests show the Postgres guard pattern — reuse it rather than adding a second.
- Pure functions in the package are the cheapest coverage and need no database; start there.
- Measure with -coverprofile and quote both numbers; an unmeasured claim is not checkable.

---

## 4. test(meter): raise measured coverage of internal/meter

`complexity: medium` · 255 points · area `meter`

**What's wrong.** `go test ./internal/... -cover` reports **21.0% for internal/meter** against 1386 lines. CLAUDE.md §8 requires "every error path tested, not just happy paths"; at this level most of them are not. Part of the gap is that the Postgres-backed tests skip without Docker, which means the real number is only ever observed in CI and nobody sees a regression locally.

**Proposed fix.** `internal/meter` measures 21.0% across 1386 lines of source. Bring its pricing, period closing and anchoring under test, concentrating on the error paths rather than the happy one.

**Acceptance criteria**

1. `go test ./internal/meter/ -cover` improves by at least 20 percentage points with Postgres available
2. Every exported function in the package has at least one test reaching its error path
3. Before and after coverage numbers are recorded in the PR description
4. No test passes by skipping; scripts/test-ci.sh fails the build on a skip

**Guidance**

- internal/meter's existing tests show the Postgres guard pattern — reuse it rather than adding a second.
- Pure functions in the package are the cheapest coverage and need no database; start there.
- Measure with -coverprofile and quote both numbers; an unmeasured claim is not checkable.

---

## 5. test(store): raise measured coverage of internal/store

`complexity: medium` · 255 points · area `store`

**What's wrong.** `go test ./internal/... -cover` reports **28.0% for internal/store** against 299 lines. CLAUDE.md §8 requires "every error path tested, not just happy paths"; at this level most of them are not. Part of the gap is that the Postgres-backed tests skip without Docker, which means the real number is only ever observed in CI and nobody sees a regression locally.

**Proposed fix.** `internal/store` measures 28.0% across 299 lines of source. Bring its migrations and request persistence under test, concentrating on the error paths rather than the happy one.

**Acceptance criteria**

1. `go test ./internal/store/ -cover` improves by at least 20 percentage points with Postgres available
2. Every exported function in the package has at least one test reaching its error path
3. Before and after coverage numbers are recorded in the PR description
4. No test passes by skipping; scripts/test-ci.sh fails the build on a skip

**Guidance**

- internal/store's existing tests show the Postgres guard pattern — reuse it rather than adding a second.
- Pure functions in the package are the cheapest coverage and need no database; start there.
- Measure with -coverprofile and quote both numbers; an unmeasured claim is not checkable.

---

## 6. test(custody): raise measured coverage of internal/custody

`complexity: medium` · 255 points · area `custody`

**What's wrong.** `go test ./internal/... -cover` reports **37.1% for internal/custody** against 276 lines. CLAUDE.md §8 requires "every error path tested, not just happy paths"; at this level most of them are not. Part of the gap is that the Postgres-backed tests skip without Docker, which means the real number is only ever observed in CI and nobody sees a regression locally.

**Proposed fix.** `internal/custody` measures 37.1% across 276 lines of source. Bring its commitment verification and the persist-before-acknowledge path under test, concentrating on the error paths rather than the happy one.

**Acceptance criteria**

1. `go test ./internal/custody/ -cover` improves by at least 20 percentage points with Postgres available
2. Every exported function in the package has at least one test reaching its error path
3. Before and after coverage numbers are recorded in the PR description
4. No test passes by skipping; scripts/test-ci.sh fails the build on a skip

**Guidance**

- internal/custody's existing tests show the Postgres guard pattern — reuse it rather than adding a second.
- Pure functions in the package are the cheapest coverage and need no database; start there.
- Measure with -coverprofile and quote both numbers; an unmeasured claim is not checkable.

---

## 7. refactor(workspace): stop discarding errors from deferred Close calls

`complexity: medium` · 255 points · area `workspace`

**What's wrong.** CLAUDE.md §8 is unambiguous: "Never discard an error, not even in a defer — log it." There are **12** `defer x.Close()` sites in non-test code, including internal/stellar/rpc.go:96, internal/pricebook/fetch.go:70, internal/settle/metrics.go:135 and internal/settle/daemon.go:101. On an HTTP response body a dropped Close error is usually benign; on a pgx rows handle it can mask a connection the pool then reuses, which in the settler's metrics and daemon queries is the money path.

**Proposed fix.** Twelve deferred Close calls drop their error. Capture and log each, or document per site why the error genuinely cannot matter.

**Acceptance criteria**

1. No non-test `defer x.Close()` discards its error without an accompanying comment saying why
2. pgx rows.Close sites log at WARN with the query context when Close reports an error
3. A grep-based test or CI step asserts the pattern cannot return
4. go vet and staticcheck stay clean

**Guidance**

- internal/settle/metrics.go:135 and daemon.go:101 are the two that matter most; an HTTP body Close is the benign end of the range.
- The idiom is a named return plus `defer func(){ if err := c.Close(); err != nil {...} }()` — keep it readable rather than mechanical.
- Where the error genuinely cannot matter, a one-line comment saying so satisfies §8 better than silently dropping it.

---

## 8. test(pricebook): add a fuzz target for internal/pricebook

`complexity: medium` · 255 points · area `pricebook`

**What's wrong.** The repository has 4 fuzz targets and **none of them covers internal/pricebook**. internal/pricebook/fetch.go takes a URI an operator published and reads a document off the network, so its inputs are not fully under this repository's control. CLAUDE.md §8 requires every error path tested; a fuzzer is the cheapest way to find the paths nobody enumerated.

**Proposed fix.** Add a fuzz target over the schedule fetcher and parser, asserting it never panics and that every rejection returns a named error rather than a partially-built value.

**Acceptance criteria**

1. A Fuzz function exists in internal/pricebook and runs under `go test -fuzz` with a bounded -fuzztime
2. No input causes a panic, an out-of-range index, or an unbounded allocation
3. Every rejection returns an error matched with errors.Is, never a zero value with a nil error
4. A seed corpus is committed from the package's existing test inputs

**Guidance**

- internal/pricebook's existing tests supply the seed corpus; do not invent inputs where recorded ones exist.
- The CI workflow already runs bounded fuzz smoke tests — add the new target to that list so it actually runs.
- Assert the nil-on-error contract explicitly; a non-nil result beside an error is the bug most likely hiding.

---

## 9. test(anchor): add a fuzz target for internal/anchor

`complexity: medium` · 255 points · area `anchor`

**What's wrong.** The repository has 4 fuzz targets and **none of them covers internal/anchor**. internal/anchor builds the statement that is submitted on chain, so its inputs are not fully under this repository's control. CLAUDE.md §8 requires every error path tested; a fuzzer is the cheapest way to find the paths nobody enumerated.

**Proposed fix.** Add a fuzz target over AnchorPeriod's statement assembly, asserting it never panics and that every rejection returns a named error rather than a partially-built value.

**Acceptance criteria**

1. A Fuzz function exists in internal/anchor and runs under `go test -fuzz` with a bounded -fuzztime
2. No input causes a panic, an out-of-range index, or an unbounded allocation
3. Every rejection returns an error matched with errors.Is, never a zero value with a nil error
4. A seed corpus is committed from the package's existing test inputs

**Guidance**

- internal/anchor's existing tests supply the seed corpus; do not invent inputs where recorded ones exist.
- The CI workflow already runs bounded fuzz smoke tests — add the new target to that list so it actually runs.
- Assert the nil-on-error contract explicitly; a non-nil result beside an error is the bug most likely hiding.

---

## 10. test(convert): add a fuzz target for internal/convert

`complexity: medium` · 255 points · area `convert`

**What's wrong.** The repository has 4 fuzz targets and **none of them covers internal/convert**. internal/convert converts every amount between Postgres and *big.Int, so its inputs are not fully under this repository's control. CLAUDE.md §8 requires every error path tested; a fuzzer is the cheapest way to find the paths nobody enumerated.

**Proposed fix.** Add a fuzz target over NumericToBigInt, asserting it never panics and that every rejection returns a named error rather than a partially-built value.

**Acceptance criteria**

1. A Fuzz function exists in internal/convert and runs under `go test -fuzz` with a bounded -fuzztime
2. No input causes a panic, an out-of-range index, or an unbounded allocation
3. Every rejection returns an error matched with errors.Is, never a zero value with a nil error
4. A seed corpus is committed from the package's existing test inputs

**Guidance**

- internal/convert's existing tests supply the seed corpus; do not invent inputs where recorded ones exist.
- The CI workflow already runs bounded fuzz smoke tests — add the new target to that list so it actually runs.
- Assert the nil-on-error contract explicitly; a non-nil result beside an error is the bug most likely hiding.

---

## 11. test(anchor): make internal/anchor's coverage measurable without Postgres

`complexity: medium` · 255 points · area `anchor`

**What's wrong.** `go test ./internal/... -cover` reports **0.0% for internal/anchor** despite internal/anchor/anchor_test.go being 573 lines. Every one of those tests guards on Postgres and skips without it, so on any machine without Docker the package is entirely unexercised and the coverage number is silently zero. That matters because anchoring is where CLAUDE.md §4's PeriodSpansPriceChange constraint and §6's record-intent-before-submitting rule are enforced.

**Proposed fix.** internal/anchor has 573 lines of test code yet measures 0.0% coverage, because every test skips without a database. Split the pure assembly logic from the persistence so the former is testable anywhere.

**Acceptance criteria**

1. The statement-assembly logic is reachable in a test without a database
2. `go test ./internal/anchor/ -cover` reports a non-zero number with no database present
3. The existing Postgres-backed tests still run and still pass when a database is available
4. No behaviour changes; this is a seam, not a rewrite

**Guidance**

- Follow CONTRIBUTING's convention: a package that needs a sibling defines a small interface for exactly what it needs — internal/custody.ChannelReader is the model.
- The goal is a seam, not more mocks; the Postgres tests remain the ones that prove the real path.
- Record the before and after coverage in the PR so the zero is visibly fixed.

---

## 12. refactor(stellar): remove the remaining panic from internal/stellar

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** This is the only `panic(` left in non-test code under internal/. CLAUDE.md §8 states the rule without exception: "No panics outside main startup validation. A library function returns an error." Contract IDs reach this package from TB_PRICE_BOOK_ID and TB_STATEMENT_REGISTRY_ID (§7), so a malformed one crashes a daemon instead of failing at startup naming the variable.

**Proposed fix.** internal/stellar/invoke.go:306 still panics. Return an error and thread it through the callers.

**Acceptance criteria**

1. No `panic(` remains in non-test code under internal/
2. A test passes a malformed contract ID and asserts an error is returned, naming the sentinel it matches
3. A grep-based test or CI step asserts internal/ stays panic-free
4. Callers wrap the error naming the offending input, per §8

**Guidance**

- internal/stellar/invoke.go:306 is the site; follow its callers upward, which is why the panic was there.
- Validating contract IDs in internal/config at startup may be the better half of the fix — §7 wants a clear message naming the variable.
- Do not swap the panic for a silently ignored zero value; an unparseable address must stop the call.

**Entry point:** `internal/stellar/invoke.go`

---

## 13. test(anchor): cover the ErrNoToken path

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** Every other sentinel in this repository is referenced by at least one test; `ErrNoToken` and one sibling are the only two that are not. CLAUDE.md §8 requires sentinel errors to be "matched with errors.Is" and every error path tested. An untested sentinel is a branch nobody has demonstrated is reachable, which is also how a sentinel ends up unreachable without anyone noticing.

**Proposed fix.** `ErrNoToken` is declared at internal/anchor/anchor.go:45 and no test references it. Add one asserting the condition that produces it and matching with errors.Is.

**Acceptance criteria**

1. A test drives the condition that returns ErrNoToken and matches it with errors.Is
2. The test asserts the error path, not merely that some error occurred
3. `go test ./internal/anchor/ -run ErrNoToken` passes
4. If the path turns out to be unreachable, a written finding saying so closes this instead

**Guidance**

- internal/anchor/anchor.go:45 declares it — read the surrounding function to find what produces it.
- internal/anchor/anchor_test.go holds the existing patterns; match them rather than inventing a style.
- An honest "this branch is unreachable" is a valid outcome and worth more than a test that forces it artificially.

**Entry point:** `internal/anchor/anchor.go`

---

## 14. test(anchor): cover the ErrUnknownChannel path

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** Every other sentinel in this repository is referenced by at least one test; `ErrUnknownChannel` and one sibling are the only two that are not. CLAUDE.md §8 requires sentinel errors to be "matched with errors.Is" and every error path tested. An untested sentinel is a branch nobody has demonstrated is reachable, which is also how a sentinel ends up unreachable without anyone noticing.

**Proposed fix.** `ErrUnknownChannel` is declared at internal/anchor/anchor.go:50 and no test references it. Add one asserting the condition that produces it and matching with errors.Is.

**Acceptance criteria**

1. A test drives the condition that returns ErrUnknownChannel and matches it with errors.Is
2. The test asserts the error path, not merely that some error occurred
3. `go test ./internal/anchor/ -run ErrUnknownChannel` passes
4. If the path turns out to be unreachable, a written finding saying so closes this instead

**Guidance**

- internal/anchor/anchor.go:50 declares it — read the surrounding function to find what produces it.
- internal/anchor/anchor_test.go holds the existing patterns; match them rather than inventing a style.
- An honest "this branch is unreachable" is a valid outcome and worth more than a test that forces it artificially.

**Entry point:** `internal/anchor/anchor.go`

---

## 15. docs(stellar): add doc comments to the four exported Error methods

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** CONTRIBUTING requires that "every exported identifier gets a doc comment", and 273 of this repository's 277 exported identifiers have one. The four that do not are all Error() methods: internal/stellar/xdr.go:25, internal/stellar/rpc.go:72, internal/stellar/invoke.go:35 and internal/stellar/invoke.go:46. These are the typed errors callers match with errors.As, so what they carry is exactly what a caller needs documented.

**Proposed fix.** Four exported Error() methods have no doc comment. Add one to each saying what the message contains and what a caller should match on.

**Acceptance criteria**

1. All four Error() methods carry a doc comment
2. Each says what the formatted message contains and which fields a caller should read rather than parse
3. A test or CI step asserts no exported identifier under internal/ lacks a doc comment
4. `go doc` renders each one attached to its type

**Guidance**

- internal/settle's sentinel declarations show the house style for documenting an error — follow it.
- Say what to match on: these are errors.As targets, so the fields matter more than the string.
- Comments explain why, not what — do not restate the method signature.

**Entry point:** `internal/stellar/xdr.go`

---

## 16. fix(workspace): stop discarding write errors in the health handlers

`complexity: low` · 165 points · area `workspace` · `good first issue`

**What's wrong.** CLAUDE.md §8: "Never discard an error, not even in a defer — log it." There are seven `_, _ = w.Write(...)` sites across cmd/collector/main.go:139 and :147, cmd/collector/routes.go:111, cmd/indexer/main.go:95 and :107, and cmd/settler/main.go:108 and :116. A failed write to a health endpoint is usually a disconnected prober and genuinely uninteresting — but that is a judgement worth writing down once rather than expressing as a silent discard the rule forbids.

**Proposed fix.** Seven health and readiness handlers discard the error from w.Write. Log it or document why it cannot matter.

**Acceptance criteria**

1. No `_, _ = w.Write` remains in cmd/
2. Each site either logs at DEBUG or carries a one-line comment saying why the error cannot matter
3. The handlers' existing status codes and bodies are unchanged
4. gofmt, go vet and staticcheck stay clean

**Guidance**

- cmd/collector/main.go:139 is the clearest place to start; the other six are the same shape.
- slog is already wired in each binary, so logging costs nothing new.
- Do not change what the handlers return — probers depend on the exact body and status.

**Entry point:** `cmd/collector/main.go`

---

## 17. test(httpserver): add a benchmark for internal/httpserver

`complexity: low` · 165 points · area `httpserver` · `good first issue`

**What's wrong.** The repository has 5 benchmarks and none covers internal/httpserver. Without a recorded baseline there is no way to notice a regression: a benchmark whose numbers were never written down cannot detect one. internal/httpserver sits on a path every metered request or settle tick reaches, so its per-call cost is worth knowing.

**Proposed fix.** Add a benchmark covering the shared server construction and shutdown path, with b.ReportAllocs, and record the baseline numbers in the PR.

**Acceptance criteria**

1. A Benchmark function covers the shared server construction and shutdown path
2. b.ReportAllocs is enabled and the baseline numbers appear in the PR description
3. The benchmark needs no Postgres and no network, so it runs in CI unchanged
4. b.ResetTimer excludes setup from the measurement

**Guidance**

- internal/httpserver's existing tests supply the fixtures; reuse them rather than building new ones.
- Record the baseline in the PR body — a benchmark with no written baseline cannot detect a regression.
- Keep inputs deterministic so timings are comparable between runs.

**Entry point:** `internal/httpserver`

---

## 18. test(pricebook): add a benchmark for internal/pricebook

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** The repository has 5 benchmarks and none covers internal/pricebook. Without a recorded baseline there is no way to notice a regression: a benchmark whose numbers were never written down cannot detect one. internal/pricebook sits on a path every metered request or settle tick reaches, so its per-call cost is worth knowing.

**Proposed fix.** Add a benchmark covering the schedule fetch and parse path, with b.ReportAllocs, and record the baseline numbers in the PR.

**Acceptance criteria**

1. A Benchmark function covers the schedule fetch and parse path
2. b.ReportAllocs is enabled and the baseline numbers appear in the PR description
3. The benchmark needs no Postgres and no network, so it runs in CI unchanged
4. b.ResetTimer excludes setup from the measurement

**Guidance**

- internal/pricebook's existing tests supply the fixtures; reuse them rather than building new ones.
- Record the baseline in the PR body — a benchmark with no written baseline cannot detect a regression.
- Keep inputs deterministic so timings are comparable between runs.

**Entry point:** `internal/pricebook`

---

## 19. test(convert): add a benchmark for internal/convert

`complexity: low` · 165 points · area `convert` · `good first issue`

**What's wrong.** The repository has 5 benchmarks and none covers internal/convert. Without a recorded baseline there is no way to notice a regression: a benchmark whose numbers were never written down cannot detect one. internal/convert sits on a path every metered request or settle tick reaches, so its per-call cost is worth knowing.

**Proposed fix.** Add a benchmark covering NumericToBigInt across representative exponents, with b.ReportAllocs, and record the baseline numbers in the PR.

**Acceptance criteria**

1. A Benchmark function covers NumericToBigInt across representative exponents
2. b.ReportAllocs is enabled and the baseline numbers appear in the PR description
3. The benchmark needs no Postgres and no network, so it runs in CI unchanged
4. b.ResetTimer excludes setup from the measurement

**Guidance**

- internal/convert's existing tests supply the fixtures; reuse them rather than building new ones.
- Record the baseline in the PR body — a benchmark with no written baseline cannot detect a regression.
- Keep inputs deterministic so timings are comparable between runs.

**Entry point:** `internal/convert`

---

## 20. docs(convert): add a runnable example for NumericToBigInt

`complexity: low` · 165 points · area `convert` · `good first issue`

**What's wrong.** `NumericToBigInt` is exported from `internal/convert` at internal/convert/numeric.go:23 (`func NumericToBigInt(n pgtype.Numeric) (*big.Int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is deterministic, so the example can assert its exact output and will fail the build the moment the behaviour changes — which is what makes an example worth more here than another line of prose.

**Proposed fix.** Add `ExampleNumericToBigInt` for `NumericToBigInt` (internal/convert/numeric.go:23), which calls it with a concrete literal input and asserts the result through an `// Output:` comment, so `go test` executes the example and it cannot silently rot.

**Acceptance criteria**

1. `ExampleNumericToBigInt` exists in `internal/convert/example_test.go` and has an `// Output:` comment, so `go test` executes and checks it
2. The example passes a concrete literal input rather than calling a helper that hides what is being passed
3. `go test ./internal/convert/...` passes with the asserted output matching exactly
4. `go doc internal/convert NumericToBigInt` shows the example attached to this identifier, not to the package

**Guidance**

- The declaration is at internal/convert/numeric.go:23 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/convert/numeric.go yet, so create `internal/convert/example_test.go` in package `convert` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/convert/numeric.go`

---

## 21. docs(indexer): add a runnable example for Ingestor.ConsecutiveTickFailures

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Ingestor.ConsecutiveTickFailures` is exported from `internal/indexer` at internal/indexer/ingest.go:151 (`func (ix *Ingestor) ConsecutiveTickFailures() uint64`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is deterministic, so the example can assert its exact output and will fail the build the moment the behaviour changes — which is what makes an example worth more here than another line of prose.

**Proposed fix.** Add `ExampleIngestor_ConsecutiveTickFailures` for `Ingestor.ConsecutiveTickFailures` (internal/indexer/ingest.go:151, a method on `Ingestor`), which calls it with a concrete literal input and asserts the result through an `// Output:` comment, so `go test` executes the example and it cannot silently rot.

**Acceptance criteria**

1. `ExampleIngestor_ConsecutiveTickFailures` exists in `internal/indexer/ingest_test.go` and has an `// Output:` comment, so `go test` executes and checks it
2. The example passes a concrete literal input rather than calling a helper that hides what is being passed
3. `go test ./internal/indexer/...` passes with the asserted output matching exactly
4. `go doc internal/indexer Ingestor.ConsecutiveTickFailures` shows the example attached to this identifier, not to the package

**Guidance**

- The declaration is at internal/indexer/ingest.go:151 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/indexer/ingest_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleIngestor_ConsecutiveTickFailures` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleConsecutiveTickFailures` would silently document the package instead of `Ingestor.ConsecutiveTickFailures`.

**Entry point:** `internal/indexer/ingest.go`

---

## 22. docs(indexer): add a runnable example for Metrics.ServeHTTP

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Metrics.ServeHTTP` is exported from `internal/indexer` at internal/indexer/metrics.go:22 (`func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is deterministic, so the example can assert its exact output and will fail the build the moment the behaviour changes — which is what makes an example worth more here than another line of prose.

**Proposed fix.** Add `ExampleMetrics_ServeHTTP` for `Metrics.ServeHTTP` (internal/indexer/metrics.go:22, a method on `Metrics`), which calls it with a concrete literal input and asserts the result through an `// Output:` comment, so `go test` executes the example and it cannot silently rot.

**Acceptance criteria**

1. `ExampleMetrics_ServeHTTP` exists in `internal/indexer/example_test.go` and has an `// Output:` comment, so `go test` executes and checks it
2. The example passes a concrete literal input rather than calling a helper that hides what is being passed
3. `go test ./internal/indexer/...` passes with the asserted output matching exactly
4. `go doc internal/indexer Metrics.ServeHTTP` shows the example attached to this identifier, not to the package

**Guidance**

- The declaration is at internal/indexer/metrics.go:22 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/indexer/metrics.go yet, so create `internal/indexer/example_test.go` in package `indexer` for it.
- The name must be exactly `ExampleMetrics_ServeHTTP` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleServeHTTP` would silently document the package instead of `Metrics.ServeHTTP`.

**Entry point:** `internal/indexer/metrics.go`

---

## 23. docs(meter): add a runnable example for Catalog.Price

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Catalog.Price` is exported from `internal/meter` at internal/meter/pricing.go:144 (`func (c *Catalog) Price(ledger uint32, method, pathTemplate string, units uint64) (*PricedRequest, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is deterministic, so the example can assert its exact output and will fail the build the moment the behaviour changes — which is what makes an example worth more here than another line of prose.

**Proposed fix.** Add `ExampleCatalog_Price` for `Catalog.Price` (internal/meter/pricing.go:144, a method on `Catalog`), which calls it with a concrete literal input and asserts the result through an `// Output:` comment, so `go test` executes the example and it cannot silently rot.

**Acceptance criteria**

1. `ExampleCatalog_Price` exists in `internal/meter/pricing_test.go` and has an `// Output:` comment, so `go test` executes and checks it
2. The example passes a concrete literal input rather than calling a helper that hides what is being passed
3. `go test ./internal/meter/...` passes with the asserted output matching exactly
4. `go doc internal/meter Catalog.Price` shows the example attached to this identifier, not to the package

**Guidance**

- The declaration is at internal/meter/pricing.go:144 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/pricing_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleCatalog_Price` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExamplePrice` would silently document the package instead of `Catalog.Price`.

**Entry point:** `internal/meter/pricing.go`

---

## 24. docs(pricebook): add a runnable example for Source.TTL

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `Source.TTL` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:182 (`func (s *Source) TTL() time.Duration`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is deterministic, so the example can assert its exact output and will fail the build the moment the behaviour changes — which is what makes an example worth more here than another line of prose.

**Proposed fix.** Add `ExampleSource_TTL` for `Source.TTL` (internal/pricebook/pricebook.go:182, a method on `Source`), which calls it with a concrete literal input and asserts the result through an `// Output:` comment, so `go test` executes the example and it cannot silently rot.

**Acceptance criteria**

1. `ExampleSource_TTL` exists in `internal/pricebook/pricebook_test.go` and has an `// Output:` comment, so `go test` executes and checks it
2. The example passes a concrete literal input rather than calling a helper that hides what is being passed
3. `go test ./internal/pricebook/...` passes with the asserted output matching exactly
4. `go doc internal/pricebook Source.TTL` shows the example attached to this identifier, not to the package

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:182 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSource_TTL` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleTTL` would silently document the package instead of `Source.TTL`.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 25. docs(anchor): add a runnable example for NewDriver

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `NewDriver` is exported from `internal/anchor` at internal/anchor/anchor.go:102 (`func NewDriver(cfg Config) (*Driver, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is a parse or construction entry point, so the valuable half of the example is the rejection: showing which sentinel a malformed input produces documents the contract far better than the signature does.

**Proposed fix.** Add `ExampleNewDriver` for `NewDriver` (internal/anchor/anchor.go:102), which shows one accepted input and one rejected input, with the rejection's error identified by the sentinel it matches.

**Acceptance criteria**

1. `ExampleNewDriver` exists in `internal/anchor/anchor_test.go` and covers one accepted and one rejected input
2. The rejected case identifies the sentinel via `errors.Is`, not by comparing an error string
3. The example has an `// Output:` comment so `go test` executes and checks it
4. `go test ./internal/anchor/...` passes

**Guidance**

- The declaration is at internal/anchor/anchor.go:102 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/anchor/anchor.go`

---

## 26. docs(indexer): add a runnable example for NewMetrics

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `NewMetrics` is exported from `internal/indexer` at internal/indexer/metrics.go:16 (`func NewMetrics(ingestor *Ingestor) *Metrics`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is a parse or construction entry point, so the valuable half of the example is the rejection: showing which sentinel a malformed input produces documents the contract far better than the signature does.

**Proposed fix.** Add `ExampleNewMetrics` for `NewMetrics` (internal/indexer/metrics.go:16), which shows one accepted input and one rejected input, with the rejection's error identified by the sentinel it matches.

**Acceptance criteria**

1. `ExampleNewMetrics` exists in `internal/indexer/example_test.go` and covers one accepted and one rejected input
2. The rejected case identifies the sentinel via `errors.Is`, not by comparing an error string
3. The example has an `// Output:` comment so `go test` executes and checks it
4. `go test ./internal/indexer/...` passes

**Guidance**

- The declaration is at internal/indexer/metrics.go:16 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/indexer/metrics.go yet, so create `internal/indexer/example_test.go` in package `indexer` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/indexer/metrics.go`

---

## 27. docs(pricebook): add a runnable example for NewHTTPFetcher

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `NewHTTPFetcher` is exported from `internal/pricebook` at internal/pricebook/fetch.go:35 (`func NewHTTPFetcher(client *http.Client) *HTTPFetcher`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is a parse or construction entry point, so the valuable half of the example is the rejection: showing which sentinel a malformed input produces documents the contract far better than the signature does.

**Proposed fix.** Add `ExampleNewHTTPFetcher` for `NewHTTPFetcher` (internal/pricebook/fetch.go:35), which shows one accepted input and one rejected input, with the rejection's error identified by the sentinel it matches.

**Acceptance criteria**

1. `ExampleNewHTTPFetcher` exists in `internal/pricebook/example_test.go` and covers one accepted and one rejected input
2. The rejected case identifies the sentinel via `errors.Is`, not by comparing an error string
3. The example has an `// Output:` comment so `go test` executes and checks it
4. `go test ./internal/pricebook/...` passes

**Guidance**

- The declaration is at internal/pricebook/fetch.go:35 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/pricebook/fetch.go yet, so create `internal/pricebook/example_test.go` in package `pricebook` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/pricebook/fetch.go`

---

## 28. docs(pricebook): add a runnable example for NewSource

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `NewSource` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:156 (`func NewSource(cfg Config) (*Source, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is a parse or construction entry point, so the valuable half of the example is the rejection: showing which sentinel a malformed input produces documents the contract far better than the signature does.

**Proposed fix.** Add `ExampleNewSource` for `NewSource` (internal/pricebook/pricebook.go:156), which shows one accepted input and one rejected input, with the rejection's error identified by the sentinel it matches.

**Acceptance criteria**

1. `ExampleNewSource` exists in `internal/pricebook/pricebook_test.go` and covers one accepted and one rejected input
2. The rejected case identifies the sentinel via `errors.Is`, not by comparing an error string
3. The example has an `// Output:` comment so `go test` executes and checks it
4. `go test ./internal/pricebook/...` passes

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:156 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 29. docs(anchor): add a runnable example for Config

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `Config` is exported from `internal/anchor` at internal/anchor/anchor.go:63 (`type Config struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleConfig` for `Config` (internal/anchor/anchor.go:63), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleConfig` exists in `internal/anchor/anchor_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/anchor/...` passes

**Guidance**

- The declaration is at internal/anchor/anchor.go:63 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/anchor/anchor.go`

---

## 30. docs(anchor): add a runnable example for Driver

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `Driver` is exported from `internal/anchor` at internal/anchor/anchor.go:90 (`type Driver struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleDriver` for `Driver` (internal/anchor/anchor.go:90), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleDriver` exists in `internal/anchor/anchor_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/anchor/...` passes

**Guidance**

- The declaration is at internal/anchor/anchor.go:90 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/anchor/anchor.go`

---

## 31. docs(httpapi): add a runnable example for PeriodOpener

`complexity: low` · 165 points · area `httpapi` · `good first issue`

**What's wrong.** `PeriodOpener` is exported from `internal/httpapi` at internal/httpapi/metering.go:67 (`type PeriodOpener interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePeriodOpener` for `PeriodOpener` (internal/httpapi/metering.go:67), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePeriodOpener` exists in `internal/httpapi/metering_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/httpapi/...` passes

**Guidance**

- The declaration is at internal/httpapi/metering.go:67 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/httpapi/metering_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/httpapi/metering.go`

---

## 32. docs(httpserver): add a runnable example for Timeouts

`complexity: low` · 165 points · area `httpserver` · `good first issue`

**What's wrong.** `Timeouts` is exported from `internal/httpserver` at internal/httpserver/server.go:12 (`type Timeouts struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleTimeouts` for `Timeouts` (internal/httpserver/server.go:12), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleTimeouts` exists in `internal/httpserver/server_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/httpserver/...` passes

**Guidance**

- The declaration is at internal/httpserver/server.go:12 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/httpserver/server_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/httpserver/server.go`

---

## 33. docs(indexer): add a runnable example for Metrics

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Metrics` is exported from `internal/indexer` at internal/indexer/metrics.go:11 (`type Metrics struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleMetrics` for `Metrics` (internal/indexer/metrics.go:11), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleMetrics` exists in `internal/indexer/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/indexer/...` passes

**Guidance**

- The declaration is at internal/indexer/metrics.go:11 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/indexer/metrics.go yet, so create `internal/indexer/example_test.go` in package `indexer` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/indexer/metrics.go`

---

## 34. docs(meter): add a runnable example for AnchorCandidate

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `AnchorCandidate` is exported from `internal/meter` at internal/meter/anchor.go:364 (`type AnchorCandidate struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleAnchorCandidate` for `AnchorCandidate` (internal/meter/anchor.go:364), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleAnchorCandidate` exists in `internal/meter/anchor_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/anchor.go:364 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/anchor.go`

---

## 35. docs(meter): add a runnable example for AnchorStatus

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `AnchorStatus` is exported from `internal/meter` at internal/meter/status.go:17 (`type AnchorStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleAnchorStatus` for `AnchorStatus` (internal/meter/status.go:17), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleAnchorStatus` exists in `internal/meter/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/status.go:17 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/meter/status.go yet, so create `internal/meter/example_test.go` in package `meter` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/status.go`

---

## 36. docs(meter): add a runnable example for PeriodCloser

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodCloser` is exported from `internal/meter` at internal/meter/period.go:42 (`type PeriodCloser struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePeriodCloser` for `PeriodCloser` (internal/meter/period.go:42), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePeriodCloser` exists in `internal/meter/period_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/period.go:42 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/period.go`

---

## 37. docs(meter): add a runnable example for PeriodStatus

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodStatus` is exported from `internal/meter` at internal/meter/status.go:6 (`type PeriodStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePeriodStatus` for `PeriodStatus` (internal/meter/status.go:6), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePeriodStatus` exists in `internal/meter/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/status.go:6 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/meter/status.go yet, so create `internal/meter/example_test.go` in package `meter` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/status.go`

---

## 38. docs(meter): add a runnable example for PricedRequest

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PricedRequest` is exported from `internal/meter` at internal/meter/pricing.go:128 (`type PricedRequest struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePricedRequest` for `PricedRequest` (internal/meter/pricing.go:128), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePricedRequest` exists in `internal/meter/pricing_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/pricing.go:128 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/pricing_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/pricing.go`

---

## 39. docs(meter): add a runnable example for Pricer

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Pricer` is exported from `internal/meter` at internal/meter/pricing.go:161 (`type Pricer struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePricer` for `Pricer` (internal/meter/pricing.go:161), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePricer` exists in `internal/meter/pricing_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/pricing.go:161 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/pricing_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/pricing.go`

---

## 40. docs(meter): add a runnable example for ReconcileResult

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `ReconcileResult` is exported from `internal/meter` at internal/meter/anchor.go:569 (`type ReconcileResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleReconcileResult` for `ReconcileResult` (internal/meter/anchor.go:569), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleReconcileResult` exists in `internal/meter/anchor_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/anchor.go:569 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/anchor.go`

---

## 41. docs(meter): add a runnable example for Rule

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Rule` is exported from `internal/meter` at internal/meter/schedule.go:40 (`type Rule struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleRule` for `Rule` (internal/meter/schedule.go:40), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleRule` exists in `internal/meter/schedule_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/schedule.go:40 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/schedule_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/schedule.go`

---

## 42. docs(meter): add a runnable example for Schedule

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Schedule` is exported from `internal/meter` at internal/meter/schedule.go:53 (`type Schedule struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSchedule` for `Schedule` (internal/meter/schedule.go:53), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSchedule` exists in `internal/meter/schedule_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/schedule.go:53 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/schedule_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/schedule.go`

---

## 43. docs(meter): add a runnable example for Scope

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Scope` is exported from `internal/meter` at internal/meter/period.go:18 (`type Scope struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleScope` for `Scope` (internal/meter/period.go:18), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleScope` exists in `internal/meter/period_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/period.go:18 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/period.go`

---

## 44. docs(meter): add a runnable example for StatementReader

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `StatementReader` is exported from `internal/meter` at internal/meter/anchor.go:83 (`type StatementReader interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatementReader` for `StatementReader` (internal/meter/anchor.go:83), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatementReader` exists in `internal/meter/anchor_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/meter/...` passes

**Guidance**

- The declaration is at internal/meter/anchor.go:83 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/meter/anchor.go`

---

## 45. docs(pricebook): add a runnable example for Config

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `Config` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:100 (`type Config struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleConfig` for `Config` (internal/pricebook/pricebook.go:100), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleConfig` exists in `internal/pricebook/pricebook_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/pricebook/...` passes

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:100 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 46. docs(pricebook): add a runnable example for HTTPFetcher

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `HTTPFetcher` is exported from `internal/pricebook` at internal/pricebook/fetch.go:25 (`type HTTPFetcher struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleHTTPFetcher` for `HTTPFetcher` (internal/pricebook/fetch.go:25), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleHTTPFetcher` exists in `internal/pricebook/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/pricebook/...` passes

**Guidance**

- The declaration is at internal/pricebook/fetch.go:25 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/pricebook/fetch.go yet, so create `internal/pricebook/example_test.go` in package `pricebook` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/pricebook/fetch.go`

---

## 47. docs(pricebook): add a runnable example for VersionReader

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `VersionReader` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:88 (`type VersionReader interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleVersionReader` for `VersionReader` (internal/pricebook/pricebook.go:88), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleVersionReader` exists in `internal/pricebook/pricebook_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/pricebook/...` passes

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:88 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 48. docs(settle): add a runnable example for AnchorStatus

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `AnchorStatus` is exported from `internal/settle` at internal/settle/status.go:40 (`type AnchorStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleAnchorStatus` for `AnchorStatus` (internal/settle/status.go:40), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleAnchorStatus` exists in `internal/settle/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/status.go:40 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/settle/status.go yet, so create `internal/settle/example_test.go` in package `settle` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/status.go`

---

## 49. docs(settle): add a runnable example for Calculator

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Calculator` is exported from `internal/settle` at internal/settle/exposure.go:62 (`type Calculator struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleCalculator` for `Calculator` (internal/settle/exposure.go:62), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleCalculator` exists in `internal/settle/exposure_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/exposure.go:62 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/exposure_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/exposure.go`

---

## 50. docs(settle): add a runnable example for ChannelFactory

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `ChannelFactory` is exported from `internal/settle` at internal/settle/daemon.go:38 (`type ChannelFactory func(address string) LiveChannel`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelFactory` for `ChannelFactory` (internal/settle/daemon.go:38), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelFactory` exists in `internal/settle/daemon_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/daemon.go:38 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/daemon.go`

---

## 51. docs(settle): add a runnable example for ChannelSettler

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `ChannelSettler` is exported from `internal/settle` at internal/settle/submit.go:40 (`type ChannelSettler interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelSettler` for `ChannelSettler` (internal/settle/submit.go:40), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelSettler` exists in `internal/settle/submit_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/submit.go:40 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/submit_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/submit.go`

---

## 52. docs(settle): add a runnable example for ChannelState

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `ChannelState` is exported from `internal/settle` at internal/settle/policy.go:43 (`type ChannelState struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelState` for `ChannelState` (internal/settle/policy.go:43), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelState` exists in `internal/settle/policy_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/policy.go:43 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/policy_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/policy.go`

---

## 53. docs(settle): add a runnable example for ChannelStatus

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `ChannelStatus` is exported from `internal/settle` at internal/settle/status.go:6 (`type ChannelStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelStatus` for `ChannelStatus` (internal/settle/status.go:6), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelStatus` exists in `internal/settle/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/status.go:6 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/settle/status.go yet, so create `internal/settle/example_test.go` in package `settle` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/status.go`

---

## 54. docs(settle): add a runnable example for ChannelWithdrawnReader

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `ChannelWithdrawnReader` is exported from `internal/settle` at internal/settle/submit.go:47 (`type ChannelWithdrawnReader interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelWithdrawnReader` for `ChannelWithdrawnReader` (internal/settle/submit.go:47), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelWithdrawnReader` exists in `internal/settle/submit_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/submit.go:47 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/submit_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/submit.go`

---

## 55. docs(settle): add a runnable example for Daemon

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Daemon` is exported from `internal/settle` at internal/settle/daemon.go:60 (`type Daemon struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleDaemon` for `Daemon` (internal/settle/daemon.go:60), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleDaemon` exists in `internal/settle/daemon_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/daemon.go:60 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/daemon.go`

---

## 56. docs(settle): add a runnable example for Decision

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Decision` is exported from `internal/settle` at internal/settle/policy.go:51 (`type Decision struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleDecision` for `Decision` (internal/settle/policy.go:51), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleDecision` exists in `internal/settle/policy_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/policy.go:51 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/policy_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/policy.go`

---

## 57. docs(settle): add a runnable example for Exposure

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Exposure` is exported from `internal/settle` at internal/settle/exposure.go:30 (`type Exposure struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleExposure` for `Exposure` (internal/settle/exposure.go:30), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleExposure` exists in `internal/settle/exposure_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/exposure.go:30 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/exposure_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/exposure.go`

---

## 58. docs(settle): add a runnable example for LedgerSource

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `LedgerSource` is exported from `internal/settle` at internal/settle/daemon.go:20 (`type LedgerSource interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleLedgerSource` for `LedgerSource` (internal/settle/daemon.go:20), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleLedgerSource` exists in `internal/settle/daemon_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/daemon.go:20 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/daemon.go`

---

## 59. docs(settle): add a runnable example for LiveChannel

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `LiveChannel` is exported from `internal/settle` at internal/settle/daemon.go:28 (`type LiveChannel interface`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleLiveChannel` for `LiveChannel` (internal/settle/daemon.go:28), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleLiveChannel` exists in `internal/settle/daemon_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/daemon.go:28 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/daemon.go`

---

## 60. docs(settle): add a runnable example for Metrics

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Metrics` is exported from `internal/settle` at internal/settle/metrics.go:33 (`type Metrics struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleMetrics` for `Metrics` (internal/settle/metrics.go:33), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleMetrics` exists in `internal/settle/metrics_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/metrics.go:33 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/metrics_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/metrics.go`

---

## 61. docs(settle): add a runnable example for PeriodStatus

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `PeriodStatus` is exported from `internal/settle` at internal/settle/status.go:29 (`type PeriodStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePeriodStatus` for `PeriodStatus` (internal/settle/status.go:29), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePeriodStatus` exists in `internal/settle/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/status.go:29 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/settle/status.go yet, so create `internal/settle/example_test.go` in package `settle` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/status.go`

---

## 62. docs(settle): add a runnable example for Policy

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Policy` is exported from `internal/settle` at internal/settle/policy.go:33 (`type Policy struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePolicy` for `Policy` (internal/settle/policy.go:33), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePolicy` exists in `internal/settle/policy_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/policy.go:33 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/policy_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/policy.go`

---

## 63. docs(settle): add a runnable example for RetryOptions

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `RetryOptions` is exported from `internal/settle` at internal/settle/retry.go:44 (`type RetryOptions struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleRetryOptions` for `RetryOptions` (internal/settle/retry.go:44), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleRetryOptions` exists in `internal/settle/retry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/retry.go:44 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/retry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/retry.go`

---

## 64. docs(settle): add a runnable example for SettlementStatus

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `SettlementStatus` is exported from `internal/settle` at internal/settle/status.go:18 (`type SettlementStatus string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSettlementStatus` for `SettlementStatus` (internal/settle/status.go:18), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSettlementStatus` exists in `internal/settle/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/status.go:18 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/settle/status.go yet, so create `internal/settle/example_test.go` in package `settle` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/status.go`

---

## 65. docs(settle): add a runnable example for Submitter

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Submitter` is exported from `internal/settle` at internal/settle/submit.go:65 (`type Submitter struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSubmitter` for `Submitter` (internal/settle/submit.go:65), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSubmitter` exists in `internal/settle/submit_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/submit.go:65 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/submit_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/submit.go`

---

## 66. docs(settle): add a runnable example for Trigger

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Trigger` is exported from `internal/settle` at internal/settle/policy.go:10 (`type Trigger string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleTrigger` for `Trigger` (internal/settle/policy.go:10), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleTrigger` exists in `internal/settle/policy_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/settle/...` passes

**Guidance**

- The declaration is at internal/settle/policy.go:10 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/policy_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/settle/policy.go`

---

## 67. docs(stellar): add a runnable example for Channel

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel` is exported from `internal/stellar` at internal/stellar/channel.go:48 (`type Channel struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannel` for `Channel` (internal/stellar/channel.go:48), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannel` exists in `internal/stellar/channel_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/channel.go:48 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/channel.go`

---

## 68. docs(stellar): add a runnable example for ChannelCloseEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ChannelCloseEvent` is exported from `internal/stellar` at internal/stellar/events.go:333 (`type ChannelCloseEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelCloseEvent` for `ChannelCloseEvent` (internal/stellar/events.go:333), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelCloseEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:333 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 69. docs(stellar): add a runnable example for ChannelOpenEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ChannelOpenEvent` is exported from `internal/stellar` at internal/stellar/events.go:280 (`type ChannelOpenEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelOpenEvent` for `ChannelOpenEvent` (internal/stellar/events.go:280), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelOpenEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:280 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 70. docs(stellar): add a runnable example for ChannelRefundEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ChannelRefundEvent` is exported from `internal/stellar` at internal/stellar/events.go:394 (`type ChannelRefundEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelRefundEvent` for `ChannelRefundEvent` (internal/stellar/events.go:394), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelRefundEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:394 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 71. docs(stellar): add a runnable example for ChannelWithdrawEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ChannelWithdrawEvent` is exported from `internal/stellar` at internal/stellar/events.go:355 (`type ChannelWithdrawEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleChannelWithdrawEvent` for `ChannelWithdrawEvent` (internal/stellar/events.go:355), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleChannelWithdrawEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:355 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 72. docs(stellar): add a runnable example for Client

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Client` is exported from `internal/stellar` at internal/stellar/rpc.go:28 (`type Client struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleClient` for `Client` (internal/stellar/rpc.go:28), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleClient` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:28 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 73. docs(stellar): add a runnable example for Dispute

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Dispute` is exported from `internal/stellar` at internal/stellar/registry.go:59 (`type Dispute struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleDispute` for `Dispute` (internal/stellar/registry.go:59), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleDispute` exists in `internal/stellar/registry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/registry.go:59 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/registry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/registry.go`

---

## 74. docs(stellar): add a runnable example for ErrSimulationFailed

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ErrSimulationFailed` is exported from `internal/stellar` at internal/stellar/invoke.go:33 (`type ErrSimulationFailed struct{ Message string }`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleErrSimulationFailed` for `ErrSimulationFailed` (internal/stellar/invoke.go:33), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleErrSimulationFailed` exists in `internal/stellar/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/invoke.go:33 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/stellar/invoke.go yet, so create `internal/stellar/example_test.go` in package `stellar` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/invoke.go`

---

## 75. docs(stellar): add a runnable example for ErrTransactionFailed

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ErrTransactionFailed` is exported from `internal/stellar` at internal/stellar/invoke.go:41 (`type ErrTransactionFailed struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleErrTransactionFailed` for `ErrTransactionFailed` (internal/stellar/invoke.go:41), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleErrTransactionFailed` exists in `internal/stellar/example_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/invoke.go:41 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/stellar/invoke.go yet, so create `internal/stellar/example_test.go` in package `stellar` for it.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/invoke.go`

---

## 76. docs(stellar): add a runnable example for ErrWrongScValType

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `ErrWrongScValType` is exported from `internal/stellar` at internal/stellar/xdr.go:20 (`type ErrWrongScValType struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleErrWrongScValType` for `ErrWrongScValType` (internal/stellar/xdr.go:20), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleErrWrongScValType` exists in `internal/stellar/xdr_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/xdr.go:20 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/xdr_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/xdr.go`

---

## 77. docs(stellar): add a runnable example for EventFilter

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `EventFilter` is exported from `internal/stellar` at internal/stellar/rpc.go:140 (`type EventFilter struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleEventFilter` for `EventFilter` (internal/stellar/rpc.go:140), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleEventFilter` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:140 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 78. docs(stellar): add a runnable example for EventInfo

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `EventInfo` is exported from `internal/stellar` at internal/stellar/rpc.go:166 (`type EventInfo struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleEventInfo` for `EventInfo` (internal/stellar/rpc.go:166), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleEventInfo` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:166 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 79. docs(stellar): add a runnable example for EventsPagination

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `EventsPagination` is exported from `internal/stellar` at internal/stellar/rpc.go:151 (`type EventsPagination struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleEventsPagination` for `EventsPagination` (internal/stellar/rpc.go:151), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleEventsPagination` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:151 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 80. docs(stellar): add a runnable example for GetEventsParams

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `GetEventsParams` is exported from `internal/stellar` at internal/stellar/rpc.go:157 (`type GetEventsParams struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleGetEventsParams` for `GetEventsParams` (internal/stellar/rpc.go:157), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleGetEventsParams` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:157 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 81. docs(stellar): add a runnable example for GetEventsResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `GetEventsResult` is exported from `internal/stellar` at internal/stellar/rpc.go:181 (`type GetEventsResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleGetEventsResult` for `GetEventsResult` (internal/stellar/rpc.go:181), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleGetEventsResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:181 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 82. docs(stellar): add a runnable example for GetLatestLedgerResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `GetLatestLedgerResult` is exported from `internal/stellar` at internal/stellar/rpc.go:119 (`type GetLatestLedgerResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleGetLatestLedgerResult` for `GetLatestLedgerResult` (internal/stellar/rpc.go:119), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleGetLatestLedgerResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:119 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 83. docs(stellar): add a runnable example for GetLedgerEntriesResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `GetLedgerEntriesResult` is exported from `internal/stellar` at internal/stellar/rpc.go:209 (`type GetLedgerEntriesResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleGetLedgerEntriesResult` for `GetLedgerEntriesResult` (internal/stellar/rpc.go:209), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleGetLedgerEntriesResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:209 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 84. docs(stellar): add a runnable example for GetTransactionResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `GetTransactionResult` is exported from `internal/stellar` at internal/stellar/rpc.go:350 (`type GetTransactionResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleGetTransactionResult` for `GetTransactionResult` (internal/stellar/rpc.go:350), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleGetTransactionResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:350 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 85. docs(stellar): add a runnable example for LedgerEntryChange

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `LedgerEntryChange` is exported from `internal/stellar` at internal/stellar/rpc.go:257 (`type LedgerEntryChange struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleLedgerEntryChange` for `LedgerEntryChange` (internal/stellar/rpc.go:257), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleLedgerEntryChange` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:257 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 86. docs(stellar): add a runnable example for LedgerEntryResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `LedgerEntryResult` is exported from `internal/stellar` at internal/stellar/rpc.go:200 (`type LedgerEntryResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleLedgerEntryResult` for `LedgerEntryResult` (internal/stellar/rpc.go:200), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleLedgerEntryResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:200 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 87. docs(stellar): add a runnable example for PriceBook

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `PriceBook` is exported from `internal/stellar` at internal/stellar/pricebook.go:42 (`type PriceBook struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePriceBook` for `PriceBook` (internal/stellar/pricebook.go:42), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePriceBook` exists in `internal/stellar/pricebook_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/pricebook.go:42 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/pricebook.go`

---

## 88. docs(stellar): add a runnable example for PriceBookPublishEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `PriceBookPublishEvent` is exported from `internal/stellar` at internal/stellar/events.go:93 (`type PriceBookPublishEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePriceBookPublishEvent` for `PriceBookPublishEvent` (internal/stellar/events.go:93), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePriceBookPublishEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:93 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 89. docs(stellar): add a runnable example for PriceBookVersion

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `PriceBookVersion` is exported from `internal/stellar` at internal/stellar/pricebook.go:23 (`type PriceBookVersion struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExamplePriceBookVersion` for `PriceBookVersion` (internal/stellar/pricebook.go:23), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExamplePriceBookVersion` exists in `internal/stellar/pricebook_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/pricebook.go:23 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/pricebook.go`

---

## 90. docs(stellar): add a runnable example for Protocol

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Protocol` is exported from `internal/stellar` at internal/stellar/registry.go:17 (`type Protocol string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleProtocol` for `Protocol` (internal/stellar/registry.go:17), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleProtocol` exists in `internal/stellar/registry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/registry.go:17 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/registry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/registry.go`

---

## 91. docs(stellar): add a runnable example for RPCError

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `RPCError` is exported from `internal/stellar` at internal/stellar/rpc.go:66 (`type RPCError struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleRPCError` for `RPCError` (internal/stellar/rpc.go:66), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleRPCError` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:66 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 92. docs(stellar): add a runnable example for RestorePreamble

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `RestorePreamble` is exported from `internal/stellar` at internal/stellar/rpc.go:251 (`type RestorePreamble struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleRestorePreamble` for `RestorePreamble` (internal/stellar/rpc.go:251), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleRestorePreamble` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:251 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 93. docs(stellar): add a runnable example for SendTransactionResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SendTransactionResult` is exported from `internal/stellar` at internal/stellar/rpc.go:309 (`type SendTransactionResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSendTransactionResult` for `SendTransactionResult` (internal/stellar/rpc.go:309), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSendTransactionResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:309 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 94. docs(stellar): add a runnable example for SimulateCost

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SimulateCost` is exported from `internal/stellar` at internal/stellar/rpc.go:265 (`type SimulateCost struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSimulateCost` for `SimulateCost` (internal/stellar/rpc.go:265), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSimulateCost` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:265 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 95. docs(stellar): add a runnable example for SimulateHostFunctionResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SimulateHostFunctionResult` is exported from `internal/stellar` at internal/stellar/rpc.go:243 (`type SimulateHostFunctionResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSimulateHostFunctionResult` for `SimulateHostFunctionResult` (internal/stellar/rpc.go:243), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSimulateHostFunctionResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:243 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 96. docs(stellar): add a runnable example for SimulateResourceConfig

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SimulateResourceConfig` is exported from `internal/stellar` at internal/stellar/rpc.go:228 (`type SimulateResourceConfig struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSimulateResourceConfig` for `SimulateResourceConfig` (internal/stellar/rpc.go:228), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSimulateResourceConfig` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:228 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 97. docs(stellar): add a runnable example for SimulateTransactionParams

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SimulateTransactionParams` is exported from `internal/stellar` at internal/stellar/rpc.go:235 (`type SimulateTransactionParams struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSimulateTransactionParams` for `SimulateTransactionParams` (internal/stellar/rpc.go:235), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSimulateTransactionParams` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:235 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 98. docs(stellar): add a runnable example for SimulateTransactionResult

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `SimulateTransactionResult` is exported from `internal/stellar` at internal/stellar/rpc.go:276 (`type SimulateTransactionResult struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleSimulateTransactionResult` for `SimulateTransactionResult` (internal/stellar/rpc.go:276), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleSimulateTransactionResult` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:276 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 99. docs(stellar): add a runnable example for Statement

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Statement` is exported from `internal/stellar` at internal/stellar/registry.go:40 (`type Statement struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatement` for `Statement` (internal/stellar/registry.go:40), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatement` exists in `internal/stellar/registry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/registry.go:40 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/registry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/registry.go`

---

## 100. docs(stellar): add a runnable example for StatementAnchorEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `StatementAnchorEvent` is exported from `internal/stellar` at internal/stellar/events.go:134 (`type StatementAnchorEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatementAnchorEvent` for `StatementAnchorEvent` (internal/stellar/events.go:134), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatementAnchorEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:134 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 101. docs(stellar): add a runnable example for StatementDisputeEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `StatementDisputeEvent` is exported from `internal/stellar` at internal/stellar/events.go:192 (`type StatementDisputeEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatementDisputeEvent` for `StatementDisputeEvent` (internal/stellar/events.go:192), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatementDisputeEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:192 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 102. docs(stellar): add a runnable example for StatementRegistry

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `StatementRegistry` is exported from `internal/stellar` at internal/stellar/registry.go:77 (`type StatementRegistry struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatementRegistry` for `StatementRegistry` (internal/stellar/registry.go:77), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatementRegistry` exists in `internal/stellar/registry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/registry.go:77 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/registry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/registry.go`

---

## 103. docs(stellar): add a runnable example for StatementResolveEvent

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `StatementResolveEvent` is exported from `internal/stellar` at internal/stellar/events.go:233 (`type StatementResolveEvent struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatementResolveEvent` for `StatementResolveEvent` (internal/stellar/events.go:233), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatementResolveEvent` exists in `internal/stellar/events_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/events.go:233 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/events_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/events.go`

---

## 104. docs(stellar): add a runnable example for Status

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Status` is exported from `internal/stellar` at internal/stellar/registry.go:29 (`type Status string`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleStatus` for `Status` (internal/stellar/registry.go:29), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleStatus` exists in `internal/stellar/registry_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/registry.go:29 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/registry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/registry.go`

---

## 105. docs(stellar): add a runnable example for TransactionEvents

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `TransactionEvents` is exported from `internal/stellar` at internal/stellar/rpc.go:340 (`type TransactionEvents struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleTransactionEvents` for `TransactionEvents` (internal/stellar/rpc.go:340), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleTransactionEvents` exists in `internal/stellar/rpc_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/stellar/...` passes

**Guidance**

- The declaration is at internal/stellar/rpc.go:340 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/rpc_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/stellar/rpc.go`

---

## 106. docs(store): add a runnable example for Migration

`complexity: low` · 165 points · area `store` · `good first issue`

**What's wrong.** `Migration` is exported from `internal/store` at internal/store/migrate.go:23 (`type Migration struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleMigration` for `Migration` (internal/store/migrate.go:23), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleMigration` exists in `internal/store/migrate_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/store/...` passes

**Guidance**

- The declaration is at internal/store/migrate.go:23 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/store/migrate_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/store/migrate.go`

---

## 107. docs(store): add a runnable example for RequestStore

`complexity: low` · 165 points · area `store` · `good first issue`

**What's wrong.** `RequestStore` is exported from `internal/store` at internal/store/requests.go:18 (`type RequestStore struct`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It is an exported type with no example, so nothing in the package documentation shows which fields a caller is expected to set versus which the package fills in. That is exactly the question an example answers and a doc comment usually does not.

**Proposed fix.** Add `ExampleRequestStore` for `RequestStore` (internal/store/requests.go:18), which constructs it and uses it the way a caller actually would, so the difference between the zero value and a properly-constructed one is visible.

**Acceptance criteria**

1. `ExampleRequestStore` exists in `internal/store/requests_test.go` and constructs the type the way a caller would
2. The example sets every field a caller is required to set, and leaves the ones the package fills in alone
3. The example either asserts an `// Output:` comment, or carries a comment saying why output is not asserted
4. `go test ./internal/store/...` passes

**Guidance**

- The declaration is at internal/store/requests.go:18 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/store/requests_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- An example whose `// Output:` comment does not match is a build failure. That is the point — do not delete the comment to make it pass.

**Entry point:** `internal/store/requests.go`

---

## 108. docs(anchor): add a runnable example for Driver.AnchorDue

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `Driver.AnchorDue` is exported from `internal/anchor` at internal/anchor/anchor.go:166 (`func (d *Driver) AnchorDue(ctx context.Context) (int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleDriver_AnchorDue` for `Driver.AnchorDue` (internal/anchor/anchor.go:166, a method on `Driver`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleDriver_AnchorDue` exists in `internal/anchor/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/anchor/...` passes without Postgres running

**Guidance**

- The declaration is at internal/anchor/anchor.go:166 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleDriver_AnchorDue` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleAnchorDue` would silently document the package instead of `Driver.AnchorDue`.

**Entry point:** `internal/anchor/anchor.go`

---

## 109. docs(anchor): add a runnable example for Driver.ReconcileUnresolved

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `Driver.ReconcileUnresolved` is exported from `internal/anchor` at internal/anchor/anchor.go:133 (`func (d *Driver) ReconcileUnresolved(ctx context.Context) (int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleDriver_ReconcileUnresolved` for `Driver.ReconcileUnresolved` (internal/anchor/anchor.go:133, a method on `Driver`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleDriver_ReconcileUnresolved` exists in `internal/anchor/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/anchor/...` passes without Postgres running

**Guidance**

- The declaration is at internal/anchor/anchor.go:133 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleDriver_ReconcileUnresolved` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleReconcileUnresolved` would silently document the package instead of `Driver.ReconcileUnresolved`.

**Entry point:** `internal/anchor/anchor.go`

---

## 110. docs(anchor): add a runnable example for Driver.Run

`complexity: low` · 165 points · area `anchor` · `good first issue`

**What's wrong.** `Driver.Run` is exported from `internal/anchor` at internal/anchor/anchor.go:215 (`func (d *Driver) Run(ctx context.Context, interval time.Duration)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleDriver_Run` for `Driver.Run` (internal/anchor/anchor.go:215, a method on `Driver`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleDriver_Run` exists in `internal/anchor/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/anchor/...` passes without Postgres running

**Guidance**

- The declaration is at internal/anchor/anchor.go:215 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/anchor/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleDriver_Run` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRun` would silently document the package instead of `Driver.Run`.

**Entry point:** `internal/anchor/anchor.go`

---

## 111. docs(custody): add a runnable example for Store.Submit

`complexity: low` · 165 points · area `custody` · `good first issue`

**What's wrong.** `Store.Submit` is exported from `internal/custody` at internal/custody/store.go:121 (`func (s *Store) Submit(ctx context.Context, channel ChannelReader, sc SubmittedCommitment) (*Record, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleStore_Submit` for `Store.Submit` (internal/custody/store.go:121, a method on `Store`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleStore_Submit` exists in `internal/custody/store_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/custody/...` passes without Postgres running

**Guidance**

- The declaration is at internal/custody/store.go:121 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/custody/store_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleStore_Submit` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleSubmit` would silently document the package instead of `Store.Submit`.

**Entry point:** `internal/custody/store.go`

---

## 112. docs(custody): add a runnable example for VerifyCommitment

`complexity: low` · 165 points · area `custody` · `good first issue`

**What's wrong.** `VerifyCommitment` is exported from `internal/custody` at internal/custody/verify.go:77 (`func VerifyCommitment(ctx context.Context, channel ChannelReader, trustedSignerKey ed25519.PublicKey, amount *big.Int, signature []byte) (bool, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleVerifyCommitment` for `VerifyCommitment` (internal/custody/verify.go:77), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleVerifyCommitment` exists in `internal/custody/verify_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/custody/...` passes without Postgres running

**Guidance**

- The declaration is at internal/custody/verify.go:77 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/custody/verify_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- Reuse the stubs the package's tests already define; the narrow interfaces exist precisely so this works without a live endpoint.

**Entry point:** `internal/custody/verify.go`

---

## 113. docs(httpapi): add a runnable example for RequestInfoFromContext

`complexity: low` · 165 points · area `httpapi` · `good first issue`

**What's wrong.** `RequestInfoFromContext` is exported from `internal/httpapi` at internal/httpapi/metering.go:157 (`func RequestInfoFromContext(ctx context.Context) (RequestInfo, bool)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleRequestInfoFromContext` for `RequestInfoFromContext` (internal/httpapi/metering.go:157), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleRequestInfoFromContext` exists in `internal/httpapi/metering_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/httpapi/...` passes without Postgres running

**Guidance**

- The declaration is at internal/httpapi/metering.go:157 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/httpapi/metering_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- Reuse the stubs the package's tests already define; the narrow interfaces exist precisely so this works without a live endpoint.

**Entry point:** `internal/httpapi/metering.go`

---

## 114. docs(httpapi): add a runnable example for WithRequestInfo

`complexity: low` · 165 points · area `httpapi` · `good first issue`

**What's wrong.** `WithRequestInfo` is exported from `internal/httpapi` at internal/httpapi/metering.go:151 (`func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleWithRequestInfo` for `WithRequestInfo` (internal/httpapi/metering.go:151), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleWithRequestInfo` exists in `internal/httpapi/metering_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/httpapi/...` passes without Postgres running

**Guidance**

- The declaration is at internal/httpapi/metering.go:151 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/httpapi/metering_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- Reuse the stubs the package's tests already define; the narrow interfaces exist precisely so this works without a live endpoint.

**Entry point:** `internal/httpapi/metering.go`

---

## 115. docs(indexer): add a runnable example for Ingestor.Run

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Ingestor.Run` is exported from `internal/indexer` at internal/indexer/ingest.go:423 (`func (ix *Ingestor) Run(ctx context.Context, interval time.Duration)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleIngestor_Run` for `Ingestor.Run` (internal/indexer/ingest.go:423, a method on `Ingestor`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleIngestor_Run` exists in `internal/indexer/ingest_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/indexer/...` passes without Postgres running

**Guidance**

- The declaration is at internal/indexer/ingest.go:423 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/indexer/ingest_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleIngestor_Run` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRun` would silently document the package instead of `Ingestor.Run`.

**Entry point:** `internal/indexer/ingest.go`

---

## 116. docs(indexer): add a runnable example for Ingestor.Tick

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Ingestor.Tick` is exported from `internal/indexer` at internal/indexer/ingest.go:333 (`func (ix *Ingestor) Tick(ctx context.Context) (int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleIngestor_Tick` for `Ingestor.Tick` (internal/indexer/ingest.go:333, a method on `Ingestor`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleIngestor_Tick` exists in `internal/indexer/ingest_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/indexer/...` passes without Postgres running

**Guidance**

- The declaration is at internal/indexer/ingest.go:333 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/indexer/ingest_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleIngestor_Tick` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleTick` would silently document the package instead of `Ingestor.Tick`.

**Entry point:** `internal/indexer/ingest.go`

---

## 117. docs(indexer): add a runnable example for Reconciler.ReconcilePeriod

`complexity: low` · 165 points · area `indexer` · `good first issue`

**What's wrong.** `Reconciler.ReconcilePeriod` is exported from `internal/indexer` at internal/indexer/reconcile.go:81 (`func (r *Reconciler) ReconcilePeriod(ctx context.Context, periodID int64) (*Result, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleReconciler_ReconcilePeriod` for `Reconciler.ReconcilePeriod` (internal/indexer/reconcile.go:81, a method on `Reconciler`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleReconciler_ReconcilePeriod` exists in `internal/indexer/reconcile_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/indexer/...` passes without Postgres running

**Guidance**

- The declaration is at internal/indexer/reconcile.go:81 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/indexer/reconcile_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleReconciler_ReconcilePeriod` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleReconcilePeriod` would silently document the package instead of `Reconciler.ReconcilePeriod`.

**Entry point:** `internal/indexer/reconcile.go`

---

## 118. docs(meter): add a runnable example for Anchorer.BilledAmount

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Anchorer.BilledAmount` is exported from `internal/meter` at internal/meter/anchor.go:336 (`func (a *Anchorer) BilledAmount(ctx context.Context, periodID int64) (*big.Int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleAnchorer_BilledAmount` for `Anchorer.BilledAmount` (internal/meter/anchor.go:336, a method on `Anchorer`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleAnchorer_BilledAmount` exists in `internal/meter/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/anchor.go:336 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleAnchorer_BilledAmount` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleBilledAmount` would silently document the package instead of `Anchorer.BilledAmount`.

**Entry point:** `internal/meter/anchor.go`

---

## 119. docs(meter): add a runnable example for Anchorer.DueForAnchoring

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Anchorer.DueForAnchoring` is exported from `internal/meter` at internal/meter/anchor.go:390 (`func (a *Anchorer) DueForAnchoring(ctx context.Context) ([]AnchorCandidate, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleAnchorer_DueForAnchoring` for `Anchorer.DueForAnchoring` (internal/meter/anchor.go:390, a method on `Anchorer`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleAnchorer_DueForAnchoring` exists in `internal/meter/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/anchor.go:390 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleAnchorer_DueForAnchoring` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleDueForAnchoring` would silently document the package instead of `Anchorer.DueForAnchoring`.

**Entry point:** `internal/meter/anchor.go`

---

## 120. docs(meter): add a runnable example for Anchorer.ReconcileAnchoring

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Anchorer.ReconcileAnchoring` is exported from `internal/meter` at internal/meter/anchor.go:600 (`func (a *Anchorer) ReconcileAnchoring(ctx context.Context, periodID int64, statements StatementReader) (*ReconcileResult, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleAnchorer_ReconcileAnchoring` for `Anchorer.ReconcileAnchoring` (internal/meter/anchor.go:600, a method on `Anchorer`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleAnchorer_ReconcileAnchoring` exists in `internal/meter/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/anchor.go:600 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleAnchorer_ReconcileAnchoring` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleReconcileAnchoring` would silently document the package instead of `Anchorer.ReconcileAnchoring`.

**Entry point:** `internal/meter/anchor.go`

---

## 121. docs(meter): add a runnable example for Anchorer.UnresolvedAttempts

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `Anchorer.UnresolvedAttempts` is exported from `internal/meter` at internal/meter/anchor.go:429 (`func (a *Anchorer) UnresolvedAttempts(ctx context.Context) ([]int64, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleAnchorer_UnresolvedAttempts` for `Anchorer.UnresolvedAttempts` (internal/meter/anchor.go:429, a method on `Anchorer`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleAnchorer_UnresolvedAttempts` exists in `internal/meter/anchor_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/anchor.go:429 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/anchor_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleAnchorer_UnresolvedAttempts` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleUnresolvedAttempts` would silently document the package instead of `Anchorer.UnresolvedAttempts`.

**Entry point:** `internal/meter/anchor.go`

---

## 122. docs(meter): add a runnable example for PeriodCloser.CloseDuePeriods

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodCloser.CloseDuePeriods` is exported from `internal/meter` at internal/meter/period.go:306 (`func (pc *PeriodCloser) CloseDuePeriods(ctx context.Context, catalogs CatalogSource, currentLedger uint32, maxAge time.Duration) ([]Period, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExamplePeriodCloser_CloseDuePeriods` for `PeriodCloser.CloseDuePeriods` (internal/meter/period.go:306, a method on `PeriodCloser`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExamplePeriodCloser_CloseDuePeriods` exists in `internal/meter/period_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/period.go:306 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExamplePeriodCloser_CloseDuePeriods` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleCloseDuePeriods` would silently document the package instead of `PeriodCloser.CloseDuePeriods`.

**Entry point:** `internal/meter/period.go`

---

## 123. docs(meter): add a runnable example for PeriodCloser.ClosePeriod

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodCloser.ClosePeriod` is exported from `internal/meter` at internal/meter/period.go:118 (`func (pc *PeriodCloser) ClosePeriod(ctx context.Context, periodID int64, endLedger uint32) error`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExamplePeriodCloser_ClosePeriod` for `PeriodCloser.ClosePeriod` (internal/meter/period.go:118, a method on `PeriodCloser`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExamplePeriodCloser_ClosePeriod` exists in `internal/meter/period_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/period.go:118 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExamplePeriodCloser_ClosePeriod` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleClosePeriod` would silently document the package instead of `PeriodCloser.ClosePeriod`.

**Entry point:** `internal/meter/period.go`

---

## 124. docs(meter): add a runnable example for PeriodCloser.EnsureOpenPeriod

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodCloser.EnsureOpenPeriod` is exported from `internal/meter` at internal/meter/period.go:146 (`func (pc *PeriodCloser) EnsureOpenPeriod(ctx context.Context, scope Scope, ledger uint32, catalog *Catalog) (*Period, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExamplePeriodCloser_EnsureOpenPeriod` for `PeriodCloser.EnsureOpenPeriod` (internal/meter/period.go:146, a method on `PeriodCloser`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExamplePeriodCloser_EnsureOpenPeriod` exists in `internal/meter/period_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/period.go:146 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExamplePeriodCloser_EnsureOpenPeriod` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleEnsureOpenPeriod` would silently document the package instead of `PeriodCloser.EnsureOpenPeriod`.

**Entry point:** `internal/meter/period.go`

---

## 125. docs(meter): add a runnable example for PeriodCloser.Run

`complexity: low` · 165 points · area `meter` · `good first issue`

**What's wrong.** `PeriodCloser.Run` is exported from `internal/meter` at internal/meter/period.go:355 (`func (pc *PeriodCloser) Run(ctx context.Context, ledgers LedgerSource, catalogs CatalogSource, maxAge, interval time.Duration)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExamplePeriodCloser_Run` for `PeriodCloser.Run` (internal/meter/period.go:355, a method on `PeriodCloser`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExamplePeriodCloser_Run` exists in `internal/meter/period_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/meter/...` passes without Postgres running

**Guidance**

- The declaration is at internal/meter/period.go:355 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/meter/period_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExamplePeriodCloser_Run` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRun` would silently document the package instead of `PeriodCloser.Run`.

**Entry point:** `internal/meter/period.go`

---

## 126. docs(pricebook): add a runnable example for FetchFunc

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `FetchFunc` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:97 (`type FetchFunc func(ctx context.Context, uri string) ([]byte, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleFetchFunc` for `FetchFunc` (internal/pricebook/pricebook.go:97), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleFetchFunc` exists in `internal/pricebook/pricebook_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/pricebook/...` passes without Postgres running

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:97 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- Reuse the stubs the package's tests already define; the narrow interfaces exist precisely so this works without a live endpoint.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 127. docs(pricebook): add a runnable example for HTTPFetcher.Fetch

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `HTTPFetcher.Fetch` is exported from `internal/pricebook` at internal/pricebook/fetch.go:44 (`func (f *HTTPFetcher) Fetch(ctx context.Context, uri string) ([]byte, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleHTTPFetcher_Fetch` for `HTTPFetcher.Fetch` (internal/pricebook/fetch.go:44, a method on `HTTPFetcher`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleHTTPFetcher_Fetch` exists in `internal/pricebook/example_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/pricebook/...` passes without Postgres running

**Guidance**

- The declaration is at internal/pricebook/fetch.go:44 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- There is no `_test.go` beside internal/pricebook/fetch.go yet, so create `internal/pricebook/example_test.go` in package `pricebook` for it.
- The name must be exactly `ExampleHTTPFetcher_Fetch` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleFetch` would silently document the package instead of `HTTPFetcher.Fetch`.

**Entry point:** `internal/pricebook/fetch.go`

---

## 128. docs(pricebook): add a runnable example for Source.CatalogFor

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `Source.CatalogFor` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:212 (`func (s *Source) CatalogFor(ctx context.Context, operator string) (*meter.Catalog, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSource_CatalogFor` for `Source.CatalogFor` (internal/pricebook/pricebook.go:212, a method on `Source`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSource_CatalogFor` exists in `internal/pricebook/pricebook_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/pricebook/...` passes without Postgres running

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:212 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSource_CatalogFor` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleCatalogFor` would silently document the package instead of `Source.CatalogFor`.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 129. docs(pricebook): add a runnable example for Source.Refresh

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `Source.Refresh` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:229 (`func (s *Source) Refresh(ctx context.Context, operator string) (*meter.Catalog, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSource_Refresh` for `Source.Refresh` (internal/pricebook/pricebook.go:229, a method on `Source`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSource_Refresh` exists in `internal/pricebook/pricebook_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/pricebook/...` passes without Postgres running

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:229 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSource_Refresh` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRefresh` would silently document the package instead of `Source.Refresh`.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 130. docs(pricebook): add a runnable example for Source.Run

`complexity: low` · 165 points · area `pricebook` · `good first issue`

**What's wrong.** `Source.Run` is exported from `internal/pricebook` at internal/pricebook/pricebook.go:239 (`func (s *Source) Run(ctx context.Context)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSource_Run` for `Source.Run` (internal/pricebook/pricebook.go:239, a method on `Source`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSource_Run` exists in `internal/pricebook/pricebook_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/pricebook/...` passes without Postgres running

**Guidance**

- The declaration is at internal/pricebook/pricebook.go:239 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/pricebook/pricebook_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSource_Run` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRun` would silently document the package instead of `Source.Run`.

**Entry point:** `internal/pricebook/pricebook.go`

---

## 131. docs(settle): add a runnable example for Calculator.Calculate

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Calculator.Calculate` is exported from `internal/settle` at internal/settle/exposure.go:81 (`func (calc *Calculator) Calculate(ctx context.Context, channel string) (*Exposure, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleCalculator_Calculate` for `Calculator.Calculate` (internal/settle/exposure.go:81, a method on `Calculator`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleCalculator_Calculate` exists in `internal/settle/exposure_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/exposure.go:81 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/exposure_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleCalculator_Calculate` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleCalculate` would silently document the package instead of `Calculator.Calculate`.

**Entry point:** `internal/settle/exposure.go`

---

## 132. docs(settle): add a runnable example for Daemon.Run

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Daemon.Run` is exported from `internal/settle` at internal/settle/daemon.go:184 (`func (d *Daemon) Run(ctx context.Context, interval time.Duration)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleDaemon_Run` for `Daemon.Run` (internal/settle/daemon.go:184, a method on `Daemon`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleDaemon_Run` exists in `internal/settle/daemon_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/daemon.go:184 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleDaemon_Run` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRun` would silently document the package instead of `Daemon.Run`.

**Entry point:** `internal/settle/daemon.go`

---

## 133. docs(settle): add a runnable example for Daemon.Tick

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Daemon.Tick` is exported from `internal/settle` at internal/settle/daemon.go:128 (`func (d *Daemon) Tick(ctx context.Context) (attempted int, underDeadlinePressure bool, err error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleDaemon_Tick` for `Daemon.Tick` (internal/settle/daemon.go:128, a method on `Daemon`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleDaemon_Tick` exists in `internal/settle/daemon_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/daemon.go:128 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/daemon_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleDaemon_Tick` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleTick` would silently document the package instead of `Daemon.Tick`.

**Entry point:** `internal/settle/daemon.go`

---

## 134. docs(settle): add a runnable example for SubmitWithRetry

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `SubmitWithRetry` is exported from `internal/settle` at internal/settle/retry.go:148 (`func SubmitWithRetry(ctx context.Context, s *Submitter, channel ChannelSettler, signer *keypair.Full, channelAddress string, amount *big.Int, opts RetryOptions) (*Record, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSubmitWithRetry` for `SubmitWithRetry` (internal/settle/retry.go:148), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSubmitWithRetry` exists in `internal/settle/retry_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/retry.go:148 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/retry_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- Reuse the stubs the package's tests already define; the narrow interfaces exist precisely so this works without a live endpoint.

**Entry point:** `internal/settle/retry.go`

---

## 135. docs(settle): add a runnable example for Submitter.ReconcileSubmitting

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Submitter.ReconcileSubmitting` is exported from `internal/settle` at internal/settle/submit.go:264 (`func (s *Submitter) ReconcileSubmitting(ctx context.Context, channelAddress string, channel ChannelWithdrawnReader) ([]*Record, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSubmitter_ReconcileSubmitting` for `Submitter.ReconcileSubmitting` (internal/settle/submit.go:264, a method on `Submitter`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSubmitter_ReconcileSubmitting` exists in `internal/settle/submit_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/submit.go:264 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/submit_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSubmitter_ReconcileSubmitting` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleReconcileSubmitting` would silently document the package instead of `Submitter.ReconcileSubmitting`.

**Entry point:** `internal/settle/submit.go`

---

## 136. docs(settle): add a runnable example for Submitter.Submit

`complexity: low` · 165 points · area `settle` · `good first issue`

**What's wrong.** `Submitter.Submit` is exported from `internal/settle` at internal/settle/submit.go:155 (`func (s *Submitter) Submit(ctx context.Context, channel ChannelSettler, signer *keypair.Full, channelAddress string, amount *big.Int) (*Record, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleSubmitter_Submit` for `Submitter.Submit` (internal/settle/submit.go:155, a method on `Submitter`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleSubmitter_Submit` exists in `internal/settle/submit_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/settle/...` passes without Postgres running

**Guidance**

- The declaration is at internal/settle/submit.go:155 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/settle/submit_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleSubmitter_Submit` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleSubmit` would silently document the package instead of `Submitter.Submit`.

**Entry point:** `internal/settle/submit.go`

---

## 137. docs(stellar): add a runnable example for Channel.Balance

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.Balance` is exported from `internal/stellar` at internal/stellar/channel.go:99 (`func (c *Channel) Balance(ctx context.Context) (*big.Int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_Balance` for `Channel.Balance` (internal/stellar/channel.go:99, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_Balance` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:99 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_Balance` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleBalance` would silently document the package instead of `Channel.Balance`.

**Entry point:** `internal/stellar/channel.go`

---

## 138. docs(stellar): add a runnable example for Channel.Close

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.Close` is exported from `internal/stellar` at internal/stellar/channel.go:200 (`func (c *Channel) Close(ctx context.Context, signer *keypair.Full, amount *big.Int, sig [64]byte) (string, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_Close` for `Channel.Close` (internal/stellar/channel.go:200, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_Close` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:200 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_Close` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleClose` would silently document the package instead of `Channel.Close`.

**Entry point:** `internal/stellar/channel.go`

---

## 139. docs(stellar): add a runnable example for Channel.CloseStart

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.CloseStart` is exported from `internal/stellar` at internal/stellar/channel.go:216 (`func (c *Channel) CloseStart(ctx context.Context, signer *keypair.Full) (string, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_CloseStart` for `Channel.CloseStart` (internal/stellar/channel.go:216, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_CloseStart` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:216 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_CloseStart` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleCloseStart` would silently document the package instead of `Channel.CloseStart`.

**Entry point:** `internal/stellar/channel.go`

---

## 140. docs(stellar): add a runnable example for Channel.Deposited

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.Deposited` is exported from `internal/stellar` at internal/stellar/channel.go:94 (`func (c *Channel) Deposited(ctx context.Context) (*big.Int, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_Deposited` for `Channel.Deposited` (internal/stellar/channel.go:94, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_Deposited` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:94 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_Deposited` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleDeposited` would silently document the package instead of `Channel.Deposited`.

**Entry point:** `internal/stellar/channel.go`

---

## 141. docs(stellar): add a runnable example for Channel.From

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.From` is exported from `internal/stellar` at internal/stellar/channel.go:68 (`func (c *Channel) From(ctx context.Context) (string, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_From` for `Channel.From` (internal/stellar/channel.go:68, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_From` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:68 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_From` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleFrom` would silently document the package instead of `Channel.From`.

**Entry point:** `internal/stellar/channel.go`

---

## 142. docs(stellar): add a runnable example for Channel.PrepareCommitment

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.PrepareCommitment` is exported from `internal/stellar` at internal/stellar/channel.go:116 (`func (c *Channel) PrepareCommitment(ctx context.Context, amount *big.Int) ([]byte, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_PrepareCommitment` for `Channel.PrepareCommitment` (internal/stellar/channel.go:116, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_PrepareCommitment` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:116 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_PrepareCommitment` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExamplePrepareCommitment` would silently document the package instead of `Channel.PrepareCommitment`.

**Entry point:** `internal/stellar/channel.go`

---

## 143. docs(stellar): add a runnable example for Channel.Refund

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.Refund` is exported from `internal/stellar` at internal/stellar/channel.go:228 (`func (c *Channel) Refund(ctx context.Context, signer *keypair.Full) (string, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_Refund` for `Channel.Refund` (internal/stellar/channel.go:228, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_Refund` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:228 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_Refund` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRefund` would silently document the package instead of `Channel.Refund`.

**Entry point:** `internal/stellar/channel.go`

---

## 144. docs(stellar): add a runnable example for Channel.RefundWaitingPeriod

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.RefundWaitingPeriod` is exported from `internal/stellar` at internal/stellar/channel.go:79 (`func (c *Channel) RefundWaitingPeriod(ctx context.Context) (uint32, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_RefundWaitingPeriod` for `Channel.RefundWaitingPeriod` (internal/stellar/channel.go:79, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_RefundWaitingPeriod` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:79 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_RefundWaitingPeriod` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleRefundWaitingPeriod` would silently document the package instead of `Channel.RefundWaitingPeriod`.

**Entry point:** `internal/stellar/channel.go`

---

## 145. docs(stellar): add a runnable example for Channel.Settle

`complexity: low` · 165 points · area `stellar` · `good first issue`

**What's wrong.** `Channel.Settle` is exported from `internal/stellar` at internal/stellar/channel.go:185 (`func (c *Channel) Settle(ctx context.Context, signer *keypair.Full, amount *big.Int, sig [64]byte) (string, error)`) and has no example. The repository has **zero** `Example` functions across all 239 exported identifiers, so `go doc` shows signatures and prose but never one executable call. It takes a `context.Context` and performs I/O, so a runnable example would need a live Postgres or Soroban endpoint. A compile-only example still documents the call shape and still breaks the build if the signature changes, which is the part worth protecting.

**Proposed fix.** Add `ExampleChannel_Settle` for `Channel.Settle` (internal/stellar/channel.go:185, a method on `Channel`), which is compile-checked against the package's existing test stubs rather than a live dependency, with a comment stating why no `// Output:` is asserted.

**Acceptance criteria**

1. `ExampleChannel_Settle` exists in `internal/stellar/channel_test.go` and compiles as part of the package's tests
2. It uses the stubs already defined in the package's tests and opens no real network or database connection
3. It constructs its context explicitly with a timeout, per CLAUDE.md §8
4. It carries a comment explaining why no `// Output:` is asserted, so the omission is deliberate
5. `go test ./internal/stellar/...` passes without Postgres running

**Guidance**

- The declaration is at internal/stellar/channel.go:185 — read its doc comment first. This repository uses doc comments to record design decisions, and the example should agree with what is there rather than restate it.
- `internal/stellar/channel_test.go` already tests this file — add the example there, or in a sibling `example_test.go` in the same package. `go test` only runs `Example*` functions that live alongside the package's other tests.
- The name must be exactly `ExampleChannel_Settle` — Go attaches an example to a method only via the `ExampleType_Method` form. Naming it `ExampleSettle` would silently document the package instead of `Channel.Settle`.

**Entry point:** `internal/stellar/channel.go`

---

