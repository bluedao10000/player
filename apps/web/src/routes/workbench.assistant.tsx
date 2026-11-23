import { createFileRoute } from '@tanstack/react-router'
import { EditorDesk } from '../features/workbench/editor-desk'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/assistant')({ component: AssistantPage })

function AssistantPage() {
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  return <WorkbenchPageShell current="assistant" title="AI 助手" english="ASSISTANT" description="查询制作状态，提出计划修改，并确认需要写入的变更。"
    auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <EditorDesk embedded chatView context={{ world: search.world, plan: search.plan, task: search.task, episode: search.episode, prompt: search.prompt }} />
  </WorkbenchPageShell>
}
