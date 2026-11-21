package catalog

import _ "embed"

//go:embed media/is_published_portrait.sql
var IsPublishedPortrait string

//go:embed media/is_published_scene.sql
var IsPublishedScene string
