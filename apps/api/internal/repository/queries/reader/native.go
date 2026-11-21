package reader

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed native.queries
var nativeSource string

var (
	nativeSourceQueries = resources.Statements(nativeSource)
	PurgeChallenges     = nativeSourceQueries["PurgeChallenges"]
	CreateChallenge     = nativeSourceQueries["CreateChallenge"]
	ConsumeChallenge    = nativeSourceQueries["ConsumeChallenge"]
	LockAppleSubject    = nativeSourceQueries["LockAppleSubject"]
	FindAppleReader     = nativeSourceQueries["FindAppleReader"]
	CreateIdentity      = nativeSourceQueries["CreateIdentity"]
	LinkApple           = nativeSourceQueries["LinkApple"]
	LockWechatUnion     = nativeSourceQueries["LockWechatUnion"]
	FindWechatUnion     = nativeSourceQueries["FindWechatUnion"]
	LinkWechatUnion     = nativeSourceQueries["LinkWechatUnion"]
)
