# 🛑 STOP — Run codemap before ANY task

```bash
codemap .                     # Project structure
codemap --deps                # How files connect
codemap --diff                # What changed vs main
codemap --diff --ref <branch> # Changes vs specific branch
```

## Required Usage

**BEFORE starting any task**, run `codemap .` first.

**ALWAYS run `codemap --deps` when:**
- User asks how something works
- Refactoring or moving code
- Tracing imports or dependencies

**ALWAYS run `codemap --diff` when:**
- Reviewing or summarizing changes
- Before committing code
- User asks what changed
- Use `--ref <branch>` when comparing against something other than main

---

## Symbol Extraction Resources

**BEFORE improving symbol extraction**, consult these reference sources:

### Tier 1: HIGH Quality (Production-Ready)

| Resource | URL | Key Value |
|----------|-----|-----------|
| **tree-sitter-typescript** | github.com/tree-sitter/tree-sitter-typescript | Official grammar, `node-types.json`, `tags.scm` |
| **nvim-treesitter queries** | github.com/nvim-treesitter/nvim-treesitter/tree/master/queries | `locals.scm`, `highlights.scm` patterns |
| **ast-grep docs** | ast-grep.github.io | Meta-variable patterns, constraint filtering |
| **MCP Tree-Sitter Server** | github.com/wrale/mcp-server-tree-sitter | Token-efficient cursor-based traversal |
| **fs_query** | github.com/PatWie/fs_query | Definition vs reference distinction |
| **Sourcegraph Code Intel** | sourcegraph.com/blog/announcing-scip | Layered approach: tree-sitter + SCIP |

### Tier 2: MEDIUM Quality (Useful Reference)

| Resource | URL | Key Value |
|----------|-----|-----------|
| **Tree-sitter Playground** | tree-sitter.github.io/tree-sitter/7-playground.html | AST visualization |
| **AST Explorer** | astexplorer.net | Multi-parser AST exploration |
| **Universal Ctags** | docs.ctags.io | Baseline comparison (regex-based) |
| **aerial.nvim** | github.com/stevearc/aerial.nvim | Neovim symbol viewer patterns |

### Key Patterns from Production Tools

**Definition vs Reference tagging:**
```scheme
(function_declaration name: (identifier) @name) @definition.function
(call_expression function: (identifier) @name) @reference.call
```

**Scope boundaries:**
```scheme
(statement_block) @local.scope
(method_definition) @local.scope
```

**Identifier type awareness:**
- `type_identifier` - Types, classes, interfaces
- `property_identifier` - Methods, properties
- `identifier` - Variables, parameters

### Current Implementation

- **TypeScript:** 24 rules in `scanner/sg-rules/typescript.yml`
- **JavaScript:** 12 rules in `scanner/sg-rules/javascript.yml`
- **Handler:** `scanner/astgrep.go`

### Known Gaps vs Production Tools

1. No definition/reference distinction
2. No scope tracking
3. No identifier type differentiation
