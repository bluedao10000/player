package catalog

import _ "embed"

//go:embed worlds/scope_acceptances.sql
var ScopeAcceptances string

//go:embed worlds/discovery.sql
var Discovery string

//go:embed worlds/home_worlds.sql
var HomeWorlds string

//go:embed worlds/world.sql
var World string

//go:embed worlds/character.sql
var Character string

//go:embed worlds/chapter.sql
var Chapter string

//go:embed worlds/scene.sql
var Scene string
