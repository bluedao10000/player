package fixtures

import _ "embed"

//go:embed world/world_revision.sql
var WorldRevision string

//go:embed world/insert_preface.sql
var InsertPreface string

//go:embed world/insert_second_scene.sql
var InsertSecondScene string

//go:embed world/insert_unsupported_appearance.sql
var InsertUnsupportedAppearance string

//go:embed world/create_bebop_work.sql
var CreateBebopWork string

//go:embed world/attach_bebop_source.sql
var AttachBebopSource string

//go:embed world/set_bebop_ur_ls.sql
var SetBebopURLs string

//go:embed world/set_supplement_url.sql
var SetSupplementURL string

//go:embed world/published_cast.sql
var PublishedCast string

//go:embed world/art_completeness.sql
var ArtCompleteness string

//go:embed world/tidepost_version.sql
var TidepostVersion string

//go:embed world/remove_scene_cast.sql
var RemoveSceneCast string

//go:embed world/remove_home_slot.sql
var RemoveHomeSlot string

//go:embed world/remove_world.sql
var RemoveWorld string
