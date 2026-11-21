package reader

import _ "embed"

//go:embed library/delete_bookmark.sql
var DeleteBookmark string

//go:embed library/save_position.sql
var SavePosition string

//go:embed library/save_bookmark.sql
var SaveBookmark string

//go:embed library/state.sql
var State string

//go:embed library/choose_import.sql
var ChooseImport string
