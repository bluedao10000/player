package reader

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed wechat.queries
var wechatSource string

var (
	wechatSourceQueries = resources.Statements(wechatSource)
	PurgeWechatFlows    = wechatSourceQueries["PurgeWechatFlows"]
	StartWechatFlow     = wechatSourceQueries["StartWechatFlow"]
	LockWechatIdentity  = wechatSourceQueries["LockWechatIdentity"]
	FindWechatReader    = wechatSourceQueries["FindWechatReader"]
	LinkWechat          = wechatSourceQueries["LinkWechat"]
	ConsumeWechatFlow   = wechatSourceQueries["ConsumeWechatFlow"]
	CheckBindingSession = wechatSourceQueries["CheckBindingSession"]
)
