# Jev 在项目中的使用范围

Jev 为**待判同的作品候选**提供辅助判断。管理员通过 Gin 触发评估，后端把候选证据交给配置的服务并保存建议。当前分集取证、图片生成与视觉验图走各自的 Pi 工作流。[判同入口](../apps/api/internal/handler/knowledge/match_ai.go) · [图片验图](../apps/agent/src/media.ts)

## 请求怎样留下证据

1. 选择 `MATCH_AI_PROVIDER=typesafe`，在 API 进程配置 `TYPESAFE_API_KEY` 和 `TYPESAFE_MODEL`。管理调用还需要已配置的操作者和 Bearer token。
2. `POST /api/v1/admin/knowledge/matches/{id}/assessments` 读取候选资料并计算证据摘要，调用适配器，检查结构化输出。
3. 同一候选、证据摘要、provider、模型及提示版本共用已保存的建议。使用同路径的 `GET` 读取评估记录。改变作品归属由治理 action 执行，另记原因与操作者。

适配器变更优先运行 `go -C apps/api test ./internal/service/matching -run Jev`。[适配器](../apps/api/internal/service/matching/jev.go) · [协议测试](../apps/api/internal/service/matching/jev_test.go)

第三方原文保留在 [TypeSafe 集成参考](references/typesafe-ai.md)。其中的示例、提示和模型能力是供应商参考，项目中已接入的行为以上述代码为准。
