# Review: 2026-11-16-gorilla-mux-to-net-http — run 1

**Date:** 2026-11-16
**Artifact:** review
**Verdict:** APPROVED — no blocking findings
**Rounds:** 1
**Feature artifacts:** light-design.md
**Base:** origin/main
**Base SHA:** 21159d4c04264958274ace9ea905ba81442b6ed0
**Head SHA:** 2d310dbebeb4721e5ba6d05651c385ef52724fc7
**Reviewer model:** sonnet
**Challenger model:** opus
**Brief-writer model:** opus
**Template version:** default/0.2.0

**Plugin version:** 0.7.10
**Agent Sessions:** claude-code: 8443e3c2-604b-4939-b005-27b6d4fc51ec

---

## What changed

<!-- brief:what-changed -->
This change moves the controller and tarter HTTP servers from gorilla/mux to the Go 1.22+ `net/http` ServeMux and removes gorilla/mux from `go.mod` (light-design.md:15). All four servers now build through a shared `pkg/core` layer that adds Debug request logging and panic-to-500 recovery (light-design.md:47-52). The API contract was meant to stay the same, so pinning tests were written against gorilla first (light-design.md:17). A `core.Router` wrapper keeps gorilla-style path handling on newer Go toolchains (light-design.md:109).
<!-- /brief:what-changed -->

## Where human judgement is needed

<!-- brief:human-judgement -->
- F-1, `internal/controller/server/server.go:53`: no `Access-Control-Allow-Methods` header is sent. It was already absent under gorilla (light-design.md:104), but whether the browser SPA sends preflighted requests is context the review did not have (light-design.md:82).
- F-2, `pkg/core/api/handlers.go:22`: 404 and 405 bodies become plain text, as the design accepted (light-design.md:80); you decide whether the SPA reads those bodies.
- F-3, `internal/controller/server/routes.go:29`: HEAD on GET routes now runs the handler and returns 200, a user decision (light-design.md:81) you may want to confirm.
- F-8, `pkg/core/api/server.go:50`: a request path can forge log lines; how much this matters depends on who reads these logs.
- F-5, `pkg/core/api/router.go:25`: only controller and tarter use `core.Router` (light-design.md:109); you decide whether scaler and metrics-aggregator should too.
<!-- /brief:human-judgement -->

## What was not verified

<!-- brief:not-verified -->
The smoke-test verify line at light-design.md:72 is prose, not a command, so it did not run (exit 127) and tools-verify is a gap (Gaps, Coverage). light-design.md was withheld from every read set (Gaps). All 15 findings, F-1 to F-15, reached you unchallenged (Findings). The agent's own smoke run predates the end-review fixes (light-design.md:117).
<!-- /brief:not-verified -->

---

## Rounds

| Round | Critical | Major | Minor | Action | Fix commits |
|-------|----------|-------|-------|--------|-------------|
| 1 | 0 | 0 | 15 | stop | 0 |

## Findings

Round 1.

| ID | Severity | Verdict | Dimension | Also flagged by | Site | Summary | Effect | Evidence |
|----|----------|---------|-----------|-----------------|------|---------|--------|----------|
| F-1 | minor | not challenged | correctness | contracts | `internal/controller/server/server.go:53` | mux.CORSMethodMiddleware was removed and nothing replaces it, so Access-Control-Allow-Methods is no longer sent on any controller route (routes_test.go now asserts its absence). | A cross-origin browser client that needs a preflight for a non-simple method or header gets no Allow-Methods header and the call is blocked; plain GETs are unaffected. |  |
| F-2 | minor | not challenged | contracts | testability | `pkg/core/api/handlers.go:22` | MethodNotAllowedHandler/NotFoundHandler deleted; unmatched routes and wrong methods now return ServeMux text/plain bodies instead of the SimpleResponse JSON every other error path in the API returns. | A caller that parses error bodies of the controller or tarter API as JSON gets a decode failure instead of a message on 404 or 405. |  |
| F-8 | minor | not challenged | security | performance | `pkg/core/api/server.go:50` | Middleware logs the decoded r.URL.Path with %s into the message; a request path containing %0a or %0d decodes to a raw newline, and the zap console encoder does not escape message text, so a client can forge log lines (also at the panic Errorf line). | an unauthenticated caller can inject fake entries into the controller, tarter or scaler logs when debug logging or a panic log is on, misleading anyone reading them. |  |
| F-3 | minor | not challenged | contracts |  | `internal/controller/server/routes.go:29` | ServeMux 'GET' patterns also match HEAD, whereas Methods("GET") did not; only /ws/watch got an explicit HEAD guard, so HEAD on /nodes, /runners and the id routes now runs the handlers. | HEAD requests that used to get 405 now return 200 and execute the handler, a harmless but visible behaviour change. |  |
| F-4 | minor | not challenged | contracts |  | `pkg/core/api/server.go:87` | statusWriter.WriteHeader records any code including 1xx informational ones as the final status, so a later real status is ignored in the Debug log and recoverPanic sees status != 0 and aborts the connection instead of sending 500. | Rarely, a panicking handler that sent a 1xx header drops the connection instead of returning a 500 and logs the wrong status. |  |
| F-5 | minor | not challenged | design |  | `pkg/core/api/router.go:25` | core.Router (gorilla-style path decoding/cleaning with 301) is used only by controller and tarter, while scaler and metrics-aggregator keep a bare http.NewServeMux behind the same core.Middleware, so the shared layer has two routing behaviours for the four servers. | none |  |
| F-6 | minor | not challenged | design |  | `internal/controller/server/middleware_test.go:36` | assertDebugRequestLog is copied verbatim into four per-server middleware_test.go files although loggertest already exists as the shared test helper home; the helper belongs in pkg/logger/loggertest. | none |  |
| F-7 | minor | not challenged | design |  | `internal/tarter/api/routes.go:24` | api.NewRouter() is a one-line pass-through to core.NewRouter() that adds a second public constructor to the tarter api package; callers can use core.NewRouter directly. | none |  |
| F-9 | minor | not challenged | performance |  | `pkg/core/api/router.go:44` | Router.ServeHTTP copies the URL and the whole http.Request on every request just to clear RawPath, adding two heap allocations per request. | Small constant per-request allocation overhead; no user-visible effect at the expected request rates. |  |
| F-10 | minor | not challenged | testability |  | `internal/scaler/api/middleware_test.go:36` | assertDebugRequestLog and the panic-to-500 plus debug-log tests are copied verbatim into cmd/tarter/app, internal/controller/server, internal/metrics-aggregator/api and internal/scaler/api, although pkg/core/api/server_test.go already tests Middleware's behaviour; a change to the log format must be edited in four places. | none |  |
| F-11 | minor | not challenged | testability |  | `internal/controller/server/watcher_test.go:79` | receiveUpdates blocks on bare channel receives of f.nodes and f.runners with no timeout; if the handler never calls AddNodesWatcher or AddRunnersWatcher the test hangs until the global go test timeout instead of failing with a message, and the read deadline is only set after those receives. | none |  |
| F-12 | minor | not challenged | testability |  | `internal/controller/server/middleware_test.go:82` | Fixed time.Sleep of 2s here and 3s in watcher_test.go makes the suite slower and duplicates the same write-timeout scenario (watch through NewServer and through NewHTTPServer middleware); the sleep only waits and does not observe the deadline passing. | none |  |
| F-13 | minor | not challenged | testability |  | `internal/controller/server/routes_test.go:163` | The enable and deregister cases assert the response message 'Node n-1 disabled', which pins a misleading existing message as expected behaviour, so fixing the wording later fails the test. | none |  |
| F-14 | minor | not challenged | conventions |  | `cmd/tarter/app/server.go:149` | Tarter builds its server via unexported newHTTPServer(port, h) while controller adds exported NewHTTPServer(controller, address, port, origins) for the same job; sibling servers inline core.NewHTTPServer, so the wrapper naming/export pattern is inconsistent. | none |  |
| F-15 | minor | not challenged | conventions |  | `internal/controller/server/server.go:64` | The 'To avoid Slowloris attacks and control websocket connections' comment now sits above the whole core.NewHTTPServer call instead of beside the timeout values it explains, and the other three servers pass the same timeouts with no comment. | none |  |

## Refuted findings

None.

## Dimensions that produced no findings

- docs-coherence
- dependencies

## Coverage

| Dimension | State | Reason | Model | Findings |
|-----------|-------|--------|-------|----------|
| correctness | reviewed |  | sonnet | 1 |
| contracts | reviewed |  | sonnet | 4 |
| design | reviewed |  | sonnet | 3 |
| security | reviewed |  | sonnet | 1 |
| performance | reviewed |  | sonnet | 2 |
| testability | reviewed |  | sonnet | 5 |
| conventions | reviewed |  | sonnet | 2 |
| docs-coherence | reviewed |  | sonnet | 0 |
| dependencies | reviewed |  | sonnet | 0 |
| infra | not applicable | no .tf or .tfvars file changed | none | 0 |
| tools-verify | gap | verify command `register returns 200 with a node, heartbeat returns 200, OPTIONS carries Access-Control-Allow-Methods, the upgrade returns 101` not found (exit 127) | none | 0 |

**correctness** checked:
- pkg/core/api/router.go ServeHTTP/cleanPath: RawPath clearing, 301 redirect Location, query preservation
- pkg/core/api/server.go Middleware/recoverPanic/statusWriter: panic ordering, ErrAbortHandler re-panic, Hijack status, Flush grep (no direct Flusher users in non-test code)
- controller routes.go patterns vs old mux routes: method/OPTIONS pairs, HEAD /ws/watch vs GET specificity, /api/v1 prefix, path values in handlers
- tarter/scaler/metrics-aggregator server wiring and handler interfaces; timeouts carried over unchanged
- pkg/logger atomic swap in logger.go and internal/current; grep for remaining mux/Access-Control usages
**contracts** checked:
- pkg/core/api Middleware/statusWriter vs gorilla websocket Upgrader Hijacker use in watcher_handler.go (Hijack and Unwrap present)
- grep for http.Flusher/ResponseWriter type assertions in internal, pkg, cmd (none found)
- controller routes.go ServeMux patterns vs old mux.Methods and CORSMethodMiddleware; handlers still set Access-Control-Allow-Origin themselves
- removed SetupGenericHandlers JSON 404/405 vs pkg/clients/controller client.go (clients branch on status code, not body)
- pkg/logger current.Logger atomic swap vs all Errorf/Debugf callers and Router.ServeHTTP request clone feeding PathValue
**design** checked:
- pkg/core/api/server.go, router.go: Middleware, statusWriter, NewHTTPServer, Router surface
- controller server.go/routes.go and tarter routes.go: how handlers and routers are wired through core
- scaler and metrics-aggregator server.go: still build a bare http.NewServeMux behind core.Middleware
- pkg/logger/internal/current and loggertest: seam placement and public surface
- grep for assertDebugRequestLog: defined in four per-server middleware_test.go files
**security** checked:
- pkg/core/api/server.go Middleware and recoverPanic: what request data reaches logger.Debugf/Errorf (r.URL.Path decoded, stack trace) and how pkg/logger encodes it
- pkg/core/api/router.go ServeHTTP redirect: Location built from cleaned decoded path, cannot start with // so no open redirect to another host
- route tables in controller routes.go and tarter routes.go vs the old mux routes: no new route lacks a check its siblings have (repo has no auth middleware; OPTIONS/CORS handling unchanged in handlers)
- r.PathValue replacing mux.Vars in nodepool/runners/tarter handlers: values still pass through utils.UID and empty check; no shell or path use in diff
- go.mod/go.sum diff: only gorilla/mux removed, no dependency added; zaptest/observer is test-only
**performance** checked:
- pkg/core/api/server.go Middleware: per-request statusWriter alloc, defer chain, Debugf formatting
- pkg/core/api/router.go ServeHTTP and cleanPath: per-request URL and Request shallow copies
- pkg/logger/logger.go Debugf now atomic load; Sprintf still eager (grep Debugf callers for precedent of eager formatting)
- controller routes.go and handlers: PathValue lookups, ServeMux pattern matching, no loops or unbounded collections
- NewHTTPServer timeouts across controller, scaler, metrics-aggregator, tarter match the removed literals
**testability** checked:
- controller routes_test, servemux_test, watcher_test, middleware_test, contract_test and fake_controller_test in internal/controller/server
- core api router_test, router_surface_test and server_test (middleware panic, status, hijack, unwrap cases)
- tarter api routes_test and servemux_test plus cmd/tarter/app tests
- metrics-aggregator and scaler middleware_test and server_test additions
- grep for assertDebugRequestLog definitions across the repo: four identical copies
- loggertest and logger current-pointer tests
**conventions** checked:
- comments added in diff (grep of + // lines) vs global no-comment rule; test files carry none
- pkg/core/api/router.go, server.go, handlers.go docs against code, incl. deleted generic handlers
- grep for leftover gorilla/mux, SetupGenericHandlers, NotFoundHandler across repo and docs
- naming of NewHTTPServer/newHTTPServer and core import alias across cmd/tarter, controller, scaler, metrics-aggregator server.go
- go.mod/go.sum mux removal and test files added per package
**docs-coherence** checked:
- grep of all *.md outside the change dir for gorilla/mux: no hits
- grep of *.go for gorilla/mux imports: only comments in pkg/core/api/router.go
- go.mod/go.sum diff vs CLAUDE.md architecture and build prose: no mux claim
- CLAUDE.md and DESIGN.md/README logger and middleware mentions: none altered by the diff
**dependencies** checked:
- go.mod and go.sum diff hunks: only gorilla/mux v1.8.1 require line and its two sum lines removed
- grep gorilla/mux in non-doc go, go.mod, go.sum files: only comments in pkg/core/api/router.go remain, no import
- new test imports (testify assert/mock/require, gorilla/websocket) vs go.mod: all already direct requires at unchanged versions
- no new modules or version bumps in diff; gorilla/websocket kept, still used by new tests and the watcher

## What each reviewer was given

Diff: `origin/main...HEAD` (21159d4c..2d310dbe).

**correctness**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/contract_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/fake_controller_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server_config_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/watcher_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_surface_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest_test.go`

**contracts**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`

**design**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`

**security**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.mod`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.sum`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`

**performance**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`

**testability**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/contract_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/fake_controller_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server_config_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/watcher_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_surface_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest_test.go`

**conventions**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/cmd/tarter/app/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.mod`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.sum`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/contract_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/fake_controller_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/nodepool_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/runners_handler.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/server_config_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/controller/server/watcher_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/metrics-aggregator/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/middleware_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/scaler/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/standalone.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/handlers/sys.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/routes_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/internal/tarter/api/servemux_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/handlers.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_surface_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/router_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/core/api/server_test.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/internal/current/current.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/logger.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest.go`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/pkg/logger/loggertest/loggertest_test.go`

**dependencies**:
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.mod`
- `/Users/harchanind/code/tomtom/macos-ars-workshop/go.sum`

Withheld (under `docs/sdlc/changes/2026-11-16-gorilla-mux-to-net-http/`):
- `docs/sdlc/changes/2026-11-16-gorilla-mux-to-net-http/light-design.md`

Sent to every dimension (matched no routing rule):
- none

## Fixes

No fix wave ran.

## Gaps

- Withheld from every read set: `docs/sdlc/changes/2026-11-16-gorilla-mux-to-net-http/light-design.md` — they are part of the change, so they remain visible in the diff every reviewer reads.
- verify command `register returns 200 with a node, heartbeat returns 200, OPTIONS carries Access-Control-Allow-Methods, the upgrade returns 101` not found (exit 127)
- tools-verify is gap: verify command `register returns 200 with a node, heartbeat returns 200, OPTIONS carries Access-Control-Allow-Methods, the upgrade returns 101` not found (exit 127)
