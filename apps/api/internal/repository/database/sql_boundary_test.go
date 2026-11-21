package database_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Inspect decoded Go string literals so multiline queries, fixtures and command
// binaries are covered without matching SQL mentioned in ordinary comments.
func TestSQLLivesInDatabaseResources(t *testing.T) {
	statement := regexp.MustCompile(`(?is)^\s*(?:(?:--[^\n]*(?:\n|$)|/\*.*?\*/)\s*)*(?:select\s|with\s+(?:recursive\s+)?[\w"]+\s*(?:\([^)]*\)\s*)?as\s*\(?|insert\s+into\s|delete\s+from\s|update\s+[\w."]+\s+(?:(?:as\s+)?\w+\s+)?set\s|(?:create|alter|drop|truncate)\s+(?:(?:temporary|temp|unique|materialized)\s+)?(?:schema|table|index|view|function|trigger|sequence)\s)`)
	root := filepath.FromSlash("../../../../../")
	for _, dir := range []string{"apps/api", "apps/sync", "database"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			pos := token.NewFileSet()
			source, err := parser.ParseFile(pos, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(source, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if ok && literal.Kind == token.STRING {
					value, err := strconv.Unquote(literal.Value)
					if err == nil && statement.MatchString(value) {
						t.Errorf("%s: move the SQL statement to a database resource", pos.Position(literal.Pos()))
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
