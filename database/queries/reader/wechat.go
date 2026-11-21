package reader

import _ "embed"

//go:embed wechat/purge_wechat_flows.sql
var PurgeWechatFlows string

//go:embed wechat/start_wechat_flow.sql
var StartWechatFlow string

//go:embed wechat/lock_wechat_identity.sql
var LockWechatIdentity string

//go:embed wechat/find_wechat_reader.sql
var FindWechatReader string

//go:embed wechat/link_wechat.sql
var LinkWechat string

//go:embed wechat/consume_wechat_flow.sql
var ConsumeWechatFlow string

//go:embed wechat/check_binding_session.sql
var CheckBindingSession string
