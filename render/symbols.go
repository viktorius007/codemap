package render

import (
	"fmt"
	"sort"
	"strings"

	"codemap/scanner"
)

// Symbols renders symbol information for analyzed files
func Symbols(analyses []scanner.FileAnalysis) {
	// Sort files by path for consistent output
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].Path < analyses[j].Path
	})

	for _, a := range analyses {
		if len(a.Functions) == 0 && len(a.Structs) == 0 &&
			len(a.Interfaces) == 0 && len(a.Methods) == 0 &&
			len(a.Types) == 0 && len(a.Constants) == 0 &&
			len(a.Vars) == 0 {
			continue
		}

		fmt.Printf("\n%s%s%s\n", Cyan, a.Path, Reset)

		if len(a.Functions) > 0 {
			fmt.Printf("  %sFunctions:%s %s\n", Dim, Reset, strings.Join(a.Functions, ", "))
		}
		if len(a.Methods) > 0 {
			fmt.Printf("  %sMethods:%s %s\n", Dim, Reset, strings.Join(a.Methods, ", "))
		}
		if len(a.Structs) > 0 {
			fmt.Printf("  %sStructs:%s %s\n", Dim, Reset, strings.Join(a.Structs, ", "))
		}
		if len(a.Interfaces) > 0 {
			fmt.Printf("  %sInterfaces:%s %s\n", Dim, Reset, strings.Join(a.Interfaces, ", "))
		}
		if len(a.Types) > 0 {
			fmt.Printf("  %sTypes:%s %s\n", Dim, Reset, strings.Join(a.Types, ", "))
		}
		if len(a.Constants) > 0 {
			fmt.Printf("  %sConstants:%s %s\n", Dim, Reset, strings.Join(a.Constants, ", "))
		}
		if len(a.Vars) > 0 {
			fmt.Printf("  %sVars:%s %s\n", Dim, Reset, strings.Join(a.Vars, ", "))
		}
	}
	fmt.Println()
}
