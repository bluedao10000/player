package reader

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed library.queries
var librarySource string

var (
	librarySourceQueries = resources.Statements(librarySource)
	DeleteBookmark       = librarySourceQueries["DeleteBookmark"]
	SavePosition         = librarySourceQueries["SavePosition"]
	SaveBookmark         = librarySourceQueries["SaveBookmark"]
	State                = librarySourceQueries["State"]
	ChooseImport         = librarySourceQueries["ChooseImport"]
)
