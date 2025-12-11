package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codemap/scanner"
)

// RenderOptions controls V2 symbol rendering behavior
type RenderOptions struct {
	ShowReferences bool
	JSONOutput     bool
}

// SymbolsV2 renders rich symbol information with metadata
func SymbolsV2(analyses []scanner.FileAnalysisV2, options RenderOptions) {
	if options.JSONOutput {
		SymbolsJSON(analyses)
		return
	}

	// Sort files by path for consistent output
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].Path < analyses[j].Path
	})

	for _, a := range analyses {
		if len(a.Symbols) == 0 {
			continue
		}

		fmt.Printf("\n%s%s%s\n", Cyan, a.Path, Reset)

		// Group symbols by scope
		byScope := groupByScope(a.Symbols)

		// Sort scope keys for consistent output
		var scopes []string
		for scope := range byScope {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)

		// Process "global" first, then other scopes
		orderedScopes := make([]string, 0, len(scopes))
		for _, s := range scopes {
			if s == "global" {
				orderedScopes = append([]string{s}, orderedScopes...)
			} else {
				orderedScopes = append(orderedScopes, s)
			}
		}

		for _, scope := range orderedScopes {
			symbols := byScope[scope]

			// Print scope header if not global
			if scope != "global" {
				fmt.Printf("  %s%s%s\n", Dim, scope, Reset)
			}

			// Group by kind within scope
			byKind := groupByKind(symbols)

			// Define output order
			kindOrder := []scanner.SymbolKind{
				scanner.KindImport,
				scanner.KindClass,
				scanner.KindInterface,
				scanner.KindFunction,
				scanner.KindMethod,
				scanner.KindType,
				scanner.KindEnum,
				scanner.KindNamespace,
				scanner.KindConstant,
				scanner.KindVariable,
				scanner.KindField,
				scanner.KindProperty,
				scanner.KindDecorator,
			}

			indent := "  "
			if scope != "global" {
				indent = "    "
			}

			for _, kind := range kindOrder {
				syms, ok := byKind[kind]
				if !ok {
					continue
				}

				// Separate definitions and references
				defs := filterByRole(syms, scanner.RoleDefinition)
				refs := filterByRole(syms, scanner.RoleReference)

				if len(defs) > 0 {
					names := extractNames(defs)
					label := kindToLabel(kind)
					fmt.Printf("%s%s%s:%s %s\n", indent, Dim, label, Reset, strings.Join(names, ", "))
				}

				if options.ShowReferences && len(refs) > 0 {
					names := extractNames(refs)
					label := kindToLabel(kind) + " (refs)"
					fmt.Printf("%s%s%s:%s %s\n", indent, Dim, label, Reset, strings.Join(names, ", "))
				}
			}
		}
	}
	fmt.Println()
}

// SymbolsJSON outputs symbol information as JSON
func SymbolsJSON(analyses []scanner.FileAnalysisV2) {
	output, err := json.MarshalIndent(analyses, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(output))
}

// groupByScope groups symbols by their scope
func groupByScope(symbols []scanner.Symbol) map[string][]scanner.Symbol {
	result := make(map[string][]scanner.Symbol)
	for _, sym := range symbols {
		scope := sym.Scope
		if scope == "" {
			scope = "global"
		}
		result[scope] = append(result[scope], sym)
	}
	return result
}

// groupByKind groups symbols by their kind
func groupByKind(symbols []scanner.Symbol) map[scanner.SymbolKind][]scanner.Symbol {
	result := make(map[scanner.SymbolKind][]scanner.Symbol)
	for _, sym := range symbols {
		result[sym.Kind] = append(result[sym.Kind], sym)
	}
	return result
}

// filterByRole filters symbols by role (definition or reference)
func filterByRole(symbols []scanner.Symbol, role scanner.SymbolRole) []scanner.Symbol {
	var result []scanner.Symbol
	for _, sym := range symbols {
		if sym.Role == role {
			result = append(result, sym)
		}
	}
	return result
}

// extractNames extracts unique symbol names
func extractNames(symbols []scanner.Symbol) []string {
	seen := make(map[string]bool)
	var names []string
	for _, sym := range symbols {
		if !seen[sym.Name] {
			seen[sym.Name] = true
			names = append(names, sym.Name)
		}
	}
	return names
}

// kindToLabel converts a symbol kind to a display label
func kindToLabel(kind scanner.SymbolKind) string {
	switch kind {
	case scanner.KindFunction:
		return "Functions"
	case scanner.KindMethod:
		return "Methods"
	case scanner.KindClass:
		return "Classes"
	case scanner.KindInterface:
		return "Interfaces"
	case scanner.KindType:
		return "Types"
	case scanner.KindVariable:
		return "Vars"
	case scanner.KindConstant:
		return "Constants"
	case scanner.KindField:
		return "Fields"
	case scanner.KindProperty:
		return "Properties"
	case scanner.KindDecorator:
		return "Decorators"
	case scanner.KindImport:
		return "Imports"
	case scanner.KindNamespace:
		return "Namespaces"
	case scanner.KindEnum:
		return "Enums"
	default:
		return string(kind)
	}
}
