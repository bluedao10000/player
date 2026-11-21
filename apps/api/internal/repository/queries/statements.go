// Package queries reads named SQL fragments from embedded text catalogs.
package queries

import "strings"

func Statements(source string) map[string]string {
	result := make(map[string]string)
	for _, section := range strings.Split("\n"+source, "\n-- name: ")[1:] {
		name, query, ok := strings.Cut(section, "\n")
		if !ok || name == "" || strings.TrimSpace(query) == "" || result[name] != "" {
			panic("invalid SQL resource: " + name)
		}
		result[name] = query
	}
	return result
}
