# Agent Guidelines

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
