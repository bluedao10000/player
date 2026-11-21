package reader

import _ "embed"

//go:embed native/purge_challenges.sql
var PurgeChallenges string

//go:embed native/create_challenge.sql
var CreateChallenge string

//go:embed native/consume_challenge.sql
var ConsumeChallenge string

//go:embed native/lock_apple_subject.sql
var LockAppleSubject string

//go:embed native/find_apple_reader.sql
var FindAppleReader string

//go:embed native/create_identity.sql
var CreateIdentity string

//go:embed native/link_apple.sql
var LinkApple string

//go:embed native/lock_wechat_union.sql
var LockWechatUnion string

//go:embed native/find_wechat_union.sql
var FindWechatUnion string

//go:embed native/link_wechat_union.sql
var LinkWechatUnion string
