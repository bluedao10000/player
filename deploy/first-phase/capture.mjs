// The lease binds both source URLs and the complete registered spec to this attempt.
import { page } from '/app/apps/agent/src/research.ts'
import { readEpisodeDocuments, episodeOutline } from '/app/apps/agent/src/episode-plan.ts'

let input = ''
for await (const chunk of process.stdin) input += chunk
const job = JSON.parse(input)
try {
  job.documents = await readEpisodeDocuments(job.spec, page)
  episodeOutline(job.spec, job.documents)
} catch (error) {
  job.error = String(error.message || error).slice(0, 1000)
}
console.log(JSON.stringify(job))
