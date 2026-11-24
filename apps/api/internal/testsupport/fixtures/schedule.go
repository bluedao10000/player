package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed schedule.queries
var scheduleSource string

var (
	SetChatEditionLength      = scheduleSourceQueries["SetChatEditionLength"]
	SeedChatRetry             = scheduleSourceQueries["SeedChatRetry"]
	ChatRetryPolicy           = scheduleSourceQueries["ChatRetryPolicy"]
	ChatOperationCount        = scheduleSourceQueries["ChatOperationCount"]
	ChatTurnOperationCount    = scheduleSourceQueries["ChatTurnOperationCount"]
	ChangeChatProposalWorld   = scheduleSourceQueries["ChangeChatProposalWorld"]
	ChangeChatProposalPublish = scheduleSourceQueries["ChangeChatProposalPublish"]

	scheduleSourceQueries = resources.Statements(scheduleSource)
	CreateScheduleEdition = scheduleSourceQueries["CreateScheduleEdition"]
	CountAdmissions       = scheduleSourceQueries["CountAdmissions"]
	InsertMissingEpisodes = scheduleSourceQueries["InsertMissingEpisodes"]
	QueueStatus           = scheduleSourceQueries["QueueStatus"]
)
