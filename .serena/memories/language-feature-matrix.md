# Language Feature Support Matrix

This document tracks symbol extraction features across all supported languages and serves as a framework for achieving feature parity.

**Legend**: ✅ = Supported | ❌ = Gap (should implement) | ➖ = N/A (language doesn't have this feature) | `*` = See notes

## Feature Matrix

| Feature | TypeScript | JavaScript | Go | Python | Rust | Java | C/C++ | Others |
|---------|------------|------------|-----|--------|------|------|-------|--------|
| **Imports** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ | ✅ | ✅ | ✅ Basic |
| **Functions** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ | ✅ | ✅ | ✅ Basic |
| **Classes/Structs** | ✅ Full | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Interfaces** | ✅ | ➖ | ✅ | ➖ | ❌ | ❌ | ❌ | ❌ |
| **Methods** | ✅ Full | ✅ | ✅ | ✅ Full | ❌ | ❌ | ❌ | ❌ |
| **Types/Aliases** | ✅ | ➖ | ✅ | ➖ | ❌ | ❌ | ❌ | ❌ |
| **Enums** | ✅ | ➖ | ➖ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Constants** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Variables** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Namespaces** | ✅ | ➖ | ➖ | ➖ | ❌ | ❌ | ❌ | ❌ |
| **Decorators** | ✅ | ✅ | ➖ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Fields** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Properties** | ✅ | ✅ | ➖ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Arrow/Lambda** | ✅ | ✅ | ➖ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Generators** | ✅ | ✅ | ➖ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Scope Tracking** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Reference Tracking** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |

## Gaps in Tier 2 Languages

| Language | Gap | Description | Severity |
|----------|-----|-------------|----------|
| _None_ | — | All Tier 2 languages are at feature parity | — |

**Notes:**
- TypeScript, JavaScript, Go, and Python all have complete feature support for applicable language constructs

---

## Tier Classification

### Tier 1: Comprehensive (TypeScript)
**24 rules + 4 container rules + 3 reference rules**
- Imports (ES6, CommonJS, require)
- Functions (regular, async, generator, expressions)
- Classes (regular, abstract, with decorators)
- Interfaces, Types, Enums
- Methods (regular, signatures, getters/setters)
- Properties/Fields (with visibility modifiers)
- Namespaces/Modules
- Decorators
- Constants/Variables (const, let, var)
- Ambient declarations (declare)
- Call/Construct/Index signatures
- Static blocks
- **Scope tracking**: class, interface, namespace, enum
- **References**: function calls, new expressions, type references

### Tier 2: Comprehensive (JavaScript)
**12 rules + 1 container rule + 2 reference rules**
- Imports (ES6, CommonJS)
- Functions (regular, arrow, generator, expressions)
- Classes (with fields, methods, getters/setters)
- Private fields and methods (`#privateField`, `#privateMethod`)
- Computed property methods (`[expr]()`, `["literal"]()`)
- Decorators (Stage 3 proposal) - properly extracted even with decorators
- Constants/Variables
- Static blocks
- **Scope tracking**: class
- **References**: function calls, new expressions

**Note**: JavaScript doesn't have interfaces, type aliases, enums, or namespaces - these are TypeScript-specific features. JavaScript support is now at feature parity with TypeScript for all applicable features.

### Tier 2: Comprehensive (Go)
**10 rules + 2 container rules + 2 reference rules**
- Imports, Functions, Structs, Interfaces
- Methods (with receivers) - scope tracked via receiver type
- Type aliases (`type ID int` and `type Alias = string` both supported)
- Constants, Variables
- **Struct fields** - field declarations extracted from struct definitions
- **Scope tracking**: struct (via method receivers)
- **References**: function calls, type references

### Tier 2: Comprehensive (Python)
**12 rules + 1 container rule + 1 reference rule**
- Imports (import, from...import)
- Functions (top-level only, regular, async)
- **Methods** (class methods go to `Methods` field, not `Functions`)
- **Generators** (functions with yield)
- Classes (with decorator support, excludes Enum classes)
- **Enums** (classes inheriting from Enum → `Types` field, not `Structs`)
- Decorators (@staticmethod, @classmethod, @property, custom)
- **Properties** (`@property` getter and `@name.setter`/`@name.deleter` accessors)
- Constants (ALL_CAPS module-level assignments)
- Variables (module-level assignments, excludes lambda assignments)
- **Lambda expressions** (go to `Functions` field, not `Vars`)
- **Instance fields** (self.field = value assignments)
- **Scope tracking**: class (methods get `class:ClassName` scope)
- **References**: function calls

**Note**: Python doesn't have interfaces or type aliases as language constructs.

### Tier 4: Basic (All Others)
**2 rules each: imports + functions only**
- Rust, Java, C/C++, Ruby, Kotlin, Swift
- Scala, PHP, Lua, Elixir, Bash, Solidity, C#

---

## Gap Analysis by Language

| Language | Status | Gaps |
|----------|--------|------|
| **TypeScript** | 🟢 Complete | None |
| **JavaScript** | 🟢 Complete | None |
| **Go** | 🟢 Complete | None |
| **Python** | 🟢 Complete | None |
| **Rust** | 🔴 Basic | Structs, enums, traits, impl blocks, macros |
| **Java** | 🔴 Basic | Classes, interfaces, fields, annotations |
| **C/C++** | 🔴 Basic | Classes, structs, templates, namespaces, macros |
| **Ruby** | 🔴 Basic | Classes, modules, instance variables |
| **Kotlin** | 🔴 Basic | Classes, interfaces, data classes, properties |
| **Swift** | 🔴 Basic | Classes, structs, protocols, enums, properties |

---

## Priority for Future Development

**Future Tier 4 → Tier 2 upgrades:**
1. **Rust** - Popular systems language
2. **Java** - Enterprise applications
3. **C/C++** - Systems programming

---

## Rule Files Location

- `scanner/sg-rules/{language}.yml` - Main symbol extraction rules
- `scanner/sg-rules/{language}-containers.yml` - Scope container rules
- `scanner/sg-rules/{language}-refs.yml` - Reference tracking rules

## Adding New Features

1. Add AST node rules to `scanner/sg-rules/{language}.yml`
2. Add extraction logic to `scanner/astgrep.go` (switch cases in `ScanDirectory` and `extractSymbolV2`)
3. Add container rules if scope tracking needed
4. Add tests to `scanner/astgrep_test.go`
5. Update this matrix
