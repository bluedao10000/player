package release

import _ "embed"

//go:embed receive/lock_publication.sql
var LockPublication string

//go:embed receive/find_receipt.sql
var FindReceipt string

//go:embed receive/has_chapter_conflict.sql
var HasChapterConflict string

//go:embed receive/insert_receipt.sql
var InsertReceipt string

//go:embed receive/verify_installed_scope.sql
var VerifyInstalledScope string
