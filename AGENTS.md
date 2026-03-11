# Agent Guidelines

## File Types

- `.gsx` files are **source files** — edit these.
- `.gsx.go` files are **compiled output** — never edit these directly.

## Commit Messages

Follow this format:

```
scope: lowercase imperative description
```

- **scope** is the area of the codebase being changed (e.g. `compiler`, `cli`, `lsp`, `extension`, `e2e`)
- The description starts with a **lowercase verb in imperative mood** (e.g. `fix`, `add`, `remove`, `update`)
- No period at the end
- Keep the title line under 72 characters
- Use a blank line before the body if more detail is needed

Examples:

- `compiler: fix multi-line tag rewrite inside nested parens`
- `cli: print per-file progress to stderr`
- `lsp: remove debug logging from proxy`
- `extension: add .vscodeignore and build scripts`

## Bug Reports

When responding to a bug report, always follow this order:

1. **Start from a fresh branch** — create a new branch from an up-to-date `origin/main`.
2. **Write a failing test** — reproduce the bug with a test case that fails before any code changes.
3. **Fix the bug** — make the minimal change needed so the test passes.
4. **Open a PR** — commit both the test and the fix, then create a pull request. Do no include any product advertisement in the PR content.
