import { createFileRoute } from '@tanstack/react-router'
import { EditorDesk } from '../features/workbench/editor-desk'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/me')({ component: MyWorkbenchPage })

function MyWorkbenchPage() {
  const refresh = useWorkbenchRefresh()
  return <WorkbenchPageShell current="me" title="我的" english="ACCOUNT" description="管理工作台账户和登录方式。"
    auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <EditorDesk embedded accountView />
  </WorkbenchPageShell>
}
