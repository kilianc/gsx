# Proposal: rename the local scratch watcher to `localplay`

Status: accepted, implemented
Affects: `cmd/playground`, `playground/`, `make playground`

## Problem

Since the browser playground landed, "playground" names three unrelated things:

| Name | What it actually is | Audience |
|---|---|---|
| `cmd/playground` | Watches one file, re-runs the generator | Contributors |
| `playground/page.gsx` | A `.gsx` fixture for exercising gopls | Contributors |
| `docs/…/playground.html`, `internal/gsx/playground` | The browser playground | Readers |

Two of these are development scaffolding for people working *on* GSX. The third
is a public page for people evaluating it. They share a word and nothing else.

The cost is small but it compounds: `make playground` starts a file watcher and
has nothing to do with the playground a reader would be shown, and a stack trace
mentioning "playground" no longer says which half of the repo you are in.

## Proposal

Take `playground` for the public thing, and give the local scaffolding its own
name:

| Now | Proposed |
|---|---|
| `cmd/playground` | `cmd/localplay` |
| `playground/` (`package playground`) | `localplay/` (`package localplay`) |
| `make playground` | `make localplay` |
| `internal/gsx/playground` | unchanged |
| `cmd/gsx-wasm` | unchanged |

Nothing about the browser playground moves. This is only about vacating the
name.

## Why keep the local watcher at all

The obvious alternative is to delete it — the browser playground appears to do
the same job, better. It does not, and the reason matters:

- **The browser playground runs the committed compiler**, baked into a prebuilt
  wasm bundle. A contributor changing the compiler needs to see output from
  *their* build. `cmd/localplay` regenerates from the working tree.
- **`playground/page.gsx` is an editor fixture.** Its own comment says it exists
  so you can test go-to-definition and hover from a `.gsx` file. That check
  needs a real file on disk that gopls can see. A textarea in a browser cannot
  stand in for it.

So the two serve different people and neither replaces the other. That is
precisely why they should not share a name.

`gsx dev` does not replace it either: `gsx dev` supervises an application, and
this fixture has no application to run.

## Naming

`localplay` reads as "the playground, but local", which is the actual
relationship — same activity, different venue. That is its main advantage over
the alternatives:

- **`scratch`** — clearer about the directory's role, but says nothing about
  the connection to the playground, so it loses the one thing a reader needs to
  infer.
- **`devplay`** — collides conceptually with `gsx dev`, which is a different
  and more prominent feature.
- **Status quo** — rejected above.

`localplay` is slightly opaque on first read. That seems acceptable for a target
only contributors invoke, and the `make` help text carries the explanation.

## What changes

Small and mechanical — the only references are:

- `cmd/playground/` → `cmd/localplay/` (directory rename)
- `cmd/playground/main.go`: two hardcoded strings, the usage line and the
  `./playground` argument passed to the generator
- `playground/` → `localplay/`, and the `package playground` clause in
  `page.gsx` **and** its generated `page.gsx.go`
- `Makefile`: the `playground` target

No import path changes — neither is imported by anything, both are `main` or
standalone. `go run ./cmd/gsx -check ./...` keeps covering the fixture wherever
it lives.

## Risks

Contributors with muscle memory for `make playground` get a missing-target
error, which is self-explanatory. Nothing published changes, so there is no
external breakage and no redirect to maintain.

## Resolved

Both moved. Keeping `playground/` while the command became `cmd/localplay`
would have been less churn but would have left the most confusing name — a
top-level `playground/` that is not the playground — exactly where it was.
