package knowledge

import _ "embed"

//go:embed world_import/cache_world_page.sql
var CacheWorldPage string

//go:embed world_import/source_in_backoff.sql
var SourceInBackoff string

//go:embed world_import/skip_rate_limited_world.sql
var SkipRateLimitedWorld string

//go:embed world_import/world_manifest.sql
var WorldManifest string

//go:embed world_import/skip_unchanged_world.sql
var SkipUnchangedWorld string

//go:embed world_import/stage_world.sql
var StageWorld string

//go:embed world_import/register_world_version.sql
var RegisterWorldVersion string

//go:embed world_import/publish_world_release.sql
var PublishWorldRelease string

//go:embed world_import/save_world.sql
var SaveWorld string

//go:embed world_import/set_home_slot.sql
var SetHomeSlot string

//go:embed world_import/advance_world_version.sql
var AdvanceWorldVersion string

//go:embed world_import/finish_world_import.sql
var FinishWorldImport string

//go:embed world_import/find_tv_edition.sql
var FindTVEdition string

//go:embed world_import/linked_world_work.sql
var LinkedWorldWork string

//go:embed world_import/create_original_work.sql
var CreateOriginalWork string

//go:embed world_import/upsert_world_record.sql
var UpsertWorldRecord string

//go:embed world_import/create_affected_works.sql
var CreateAffectedWorks string

//go:embed world_import/insert_evidence.sql
var InsertEvidence string

//go:embed world_import/insert_character.sql
var InsertCharacter string

//go:embed world_import/insert_chapter.sql
var InsertChapter string

//go:embed world_import/insert_block.sql
var InsertBlock string

//go:embed world_import/insert_appearance.sql
var InsertAppearance string

//go:embed world_import/insert_setting.sql
var InsertSetting string

//go:embed world_import/insert_scene.sql
var InsertScene string

//go:embed world_import/insert_scene_appearance.sql
var InsertSceneAppearance string

//go:embed world_import/insert_storyline.sql
var InsertStoryline string

//go:embed world_import/insert_storyline_chapter.sql
var InsertStorylineChapter string
