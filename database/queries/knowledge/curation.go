package knowledge

import _ "embed"

//go:embed curation/begin_curation.sql
var BeginCuration string

//go:embed curation/read_active_field.sql
var ReadActiveField string

//go:embed curation/delete_override.sql
var DeleteOverride string

//go:embed curation/upsert_override.sql
var UpsertOverride string

//go:embed curation/mark_affected_work.sql
var MarkAffectedWork string

//go:embed curation/active_record_work.sql
var ActiveRecordWork string

//go:embed curation/match_pair.sql
var MatchPair string

//go:embed curation/last_curation_source.sql
var LastCurationSource string

//go:embed curation/clone_work.sql
var CloneWork string

//go:embed curation/mark_affected_works.sql
var MarkAffectedWorks string

//go:embed curation/find_decision_records.sql
var FindDecisionRecords string

//go:embed curation/reject_match.sql
var RejectMatch string

//go:embed curation/delete_match.sql
var DeleteMatch string

//go:embed curation/delete_identity_pin.sql
var DeleteIdentityPin string

//go:embed curation/clear_rejections.sql
var ClearRejections string

//go:embed curation/pin_records.sql
var PinRecords string

//go:embed curation/move_records.sql
var MoveRecords string

//go:embed curation/read_work_field.sql
var ReadWorkField string

//go:embed curation/record_decision.sql
var RecordDecision string
