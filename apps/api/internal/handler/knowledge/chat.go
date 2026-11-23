package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	catalog "anime-agent/api/internal/model/knowledge"
	q "anime-agent/api/internal/repository/queries/knowledge"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type chatPatch struct {
	Tool          string  `json:"tool"`
	Revision      int     `json:"expected_revision"`
	World         *string `json:"world,omitempty"`
	First         *int    `json:"first_episode,omitempty"`
	Last          *int    `json:"last_episode,omitempty"`
	Budget        *int64  `json:"token_limit,omitempty"`
	Consistency   *string `json:"character_consistency,omitempty"`
	KeepPublished *bool   `json:"preserve_published,omitempty"`
	Episode       int     `json:"episode,omitempty"`
	Instruction   *string `json:"instruction,omitempty"`
}

type chatPlan struct {
	World       string         `json:"world"`
	First       int            `json:"first_episode"`
	Last        int            `json:"last_episode"`
	Budget      int64          `json:"token_limit"`
	Consumed    int64          `json:"consumed"`
	Reserved    int64          `json:"reserved"`
	Revision    int            `json:"revision"`
	Stage       string         `json:"stage"`
	PauseReason string         `json:"paused_reason"`
	Policy      map[string]any `json:"production_policy"`
	Paused      bool           `json:"operator_paused"`
}

var readTools = map[string]string{
	"get_workspace_context": "work", "get_knowledge_completeness": "completeness", "get_production_plan": "plan", "get_episode_status": "episodes",
	"get_media_candidates": "media", "get_execution_history": "history",
}

func chatRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/chat", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		var choices, history json.RawMessage
		err := db.QueryRow(ctx, q.ChatQueries["ChatChoices"]).Scan(&choices)
		if err == nil {
			err = db.QueryRow(ctx, q.ChatQueries["ChatHistory"], c.GetString("operator")).Scan(&history)
		}
		if err != nil {
			c.Status(503)
			return
		}
		c.JSON(200, gin.H{"choices": choices, "turns": history})
	})
	g.POST("/chat", func(c *gin.Context) { startChat(c, db) })
	g.POST("/chat/:id/resolve", func(c *gin.Context) { resolveChat(c, db) })
}

func startChat(c *gin.Context, db *pgxpool.Pool) {
	var in struct {
		ID      string  `json:"request_id" binding:"required,uuid"`
		World   string  `json:"world" binding:"required,max=128"`
		Plan    *string `json:"plan_id"`
		Task    *string `json:"task_id"`
		Episode int     `json:"episode" binding:"min=0,max=100000"`
		Text    string  `json:"text" binding:"required,max=1800"`
	}
	if !editorRequest(c, &in) {
		return
	}
	if (in.Plan != nil && !catalog.ValidUUID(*in.Plan)) || (in.Task != nil && !catalog.ValidUUID(*in.Task)) || strings.TrimSpace(in.Text) == "" {
		c.Status(400)
		return
	}
	ctx := c.Request.Context()
	owner := c.GetString("operator")
	var snapshot, history json.RawMessage
	if err := db.QueryRow(ctx, q.ChatQueries["ChatContext"], in.World, in.Plan, in.Episode, in.Task).Scan(&snapshot); err != nil {
		c.JSON(404, gin.H{"error": "无法读取所选作品上下文。"})
		return
	}
	if err := db.QueryRow(ctx, q.ChatQueries["ChatHistory"], owner).Scan(&history); err != nil {
		c.Status(503)
		return
	}
	tag, err := db.Exec(ctx, q.ChatQueries["ChatStart"], in.ID, owner, in.World, in.Plan, in.Episode, in.Text, in.Task)
	if err != nil {
		c.Status(503)
		return
	}
	if tag.RowsAffected() == 0 {
		var same bool
		if err := db.QueryRow(ctx, q.ChatQueries["ChatRequestMatches"], in.ID, owner, in.World, in.Plan, in.Episode, in.Text, in.Task).Scan(&same); err != nil {
			c.Status(503)
			return
		}
		if !same {
			c.JSON(409, gin.H{"error": "请求编号已用于其他操作。"})
			return
		}
		c.Status(204)
		return
	}
	// Keep prompts bounded while retaining the latest dialogue. Database state comes from this turn's snapshot.
	var prior []struct{ World, Prompt, Reply string }
	_ = json.Unmarshal(history, &prior)
	if len(prior) > 6 {
		prior = prior[len(prior)-6:]
	}
	var sections map[string]json.RawMessage
	if err = json.Unmarshal(snapshot, &sections); err != nil {
		c.Status(503)
		return
	}
	toolNames := []string{"get_workspace_context", "get_knowledge_completeness", "get_production_plan", "get_episode_status", "get_media_candidates", "get_execution_history"}
	facts := make(map[string]any, len(toolNames))
	for _, tool := range toolNames {
		facts[tool] = sections[readTools[tool]]
	}
	facts["get_workspace_context"] = map[string]json.RawMessage{"work": sections["work"], "inventory": sections["inventory"], "editor_task": sections["editor_task"], "governance": sections["governance"]}
	input, _ := json.Marshal(map[string]any{"question": in.Text, "tools": facts, "prior": prior})
	if len(input) > 90000 {
		toolNames = nil
		_, _ = db.Exec(ctx, q.ChatQueries["ChatFinish"], in.ID, owner, "failed", "当前上下文过长，请指定分集后重试。", []string{}, nil, nil)
		c.Status(204)
		return
	}
	reply, modelErr := chatInference(ctx, in.ID, input)
	state := "answered"
	var proposal, diff any
	if modelErr != nil {
		state, reply.Text = "failed", "Pi 助手暂时无法完成本轮。请检查模型配置后重试。"
	} else if reply.Patch != nil && reply.Patch.Tool == "create_production_plan" {
		var changes map[string][2]any
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			_, _, d, e := prepareCreatePlanPatch(ctx, tx, in.World, in.Text, reply.Patch)
			changes = d
			return e
		})
		if err != nil {
			reply.Text = err.Error()
		} else {
			state, proposal, diff = "pending", reply.Patch, changes
			toolNames = append(toolNames, reply.Patch.Tool)
		}
	} else if reply.Patch != nil {
		var before chatPlan
		if in.Plan == nil || json.Unmarshal(sections["plan"], &before) != nil || before.Revision == 0 {
			reply.Text = "请先选择与当前作品匹配的制作计划，或让我先为你新建一份本地验收计划。"
		} else {
			reply.Patch.Revision = before.Revision
			var changes map[string][2]any
			err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
				_, d, _, e := prepareChatPatch(ctx, tx, *in.Plan, before, *reply.Patch)
				changes = d
				return e
			})
			if err != nil {
				reply.Text = err.Error()
			} else {
				state, proposal, diff = "pending", reply.Patch, changes
				toolNames = append(toolNames, reply.Patch.Tool)
			}
		}
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = db.Exec(finishCtx, q.ChatQueries["ChatFinish"], in.ID, owner, state, reply.Text, toolNames, proposal, diff)
	if err != nil {
		c.Status(503)
		return
	}
	c.Status(204)
}

type chatReply struct {
	Text  string     `json:"text"`
	Patch *chatPatch `json:"patch"`
}

func chatInference(ctx context.Context, id string, input []byte) (chatReply, error) {
	var reply chatReply
	base := strings.TrimRight(os.Getenv("KNOWLEDGE_RESEARCH_URL"), "/")
	if base == "" {
		return reply, errors.New("Pi endpoint unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/internal/editor/"+id+"/1/1/conversation", bytes.NewReader(input))
	if err != nil {
		return reply, err
	}
	client := http.Client{Timeout: 110 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return reply, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return reply, fmt.Errorf("Pi HTTP %d", res.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 32768)).Decode(&reply)
	if err == nil && (strings.TrimSpace(reply.Text) == "" || len(reply.Text) > 12000) {
		err = errors.New("invalid model response")
	}
	return reply, err
}

func prepareCreatePlanPatch(ctx context.Context, tx pgx.Tx, currentWorld, fallbackInstruction string, patch *chatPatch) (chatPlan, string, map[string][2]any, error) {
	var created chatPlan
	invalid := func(msg string) (chatPlan, string, map[string][2]any, error) {
		return created, "", nil, errors.New(msg)
	}
	world := strings.TrimSpace(currentWorld)
	if patch.World != nil {
		proposedWorld := strings.TrimSpace(*patch.World)
		if proposedWorld != "" && proposedWorld != world {
			return invalid("只能为当前对话的作品新建制作计划。")
		}
	}
	if world == "" || patch.First == nil || patch.Last == nil || patch.Budget == nil {
		return invalid("新建制作计划需要明确作品、起始集、截止集和总预算。")
	}
	if patch.KeepPublished != nil && !*patch.KeepPublished {
		return invalid("此计划工具必须保留已发布内容。")
	}
	consistency := "standard"
	if patch.Consistency != nil && strings.TrimSpace(*patch.Consistency) != "" {
		consistency = strings.TrimSpace(*patch.Consistency)
	}
	if consistency != "standard" && consistency != "strict" {
		return invalid("人物一致性策略仅支持标准或严格。")
	}
	first, last, budget := *patch.First, *patch.Last, *patch.Budget
	var total int
	err := tx.QueryRow(ctx, q.ChatQueries["ChatEditionSize"], world).Scan(&total)
	if err != nil {
		return invalid("无法读取当前作品的集数，请重试。")
	}
	if total == 0 {
		return invalid("请先登记作品总集数，再新建制作计划。")
	}
	if first < 1 || last < first || last-first >= 1000 || last > total {
		return invalid("集数范围超出已登记的作品版本。")
	}
	if budget < 16000 || budget > 1_000_000_000 {
		return invalid("新建计划预算需在 16,000 至 10 亿 token 之间。")
	}
	note := strings.TrimSpace(fallbackInstruction)
	if patch.Instruction != nil && strings.TrimSpace(*patch.Instruction) != "" {
		note = strings.TrimSpace(*patch.Instruction)
	}
	if note == "" {
		note = fmt.Sprintf("工作台对话新建第 %d–%d 集本地验收计划", first, last)
	}
	if runes := []rune(note); len(runes) > 1400 {
		note = string(runes[:1400])
	}
	patch.World = &world
	patch.Consistency = &consistency
	patch.Instruction = &note
	policy := map[string]any{
		"character_consistency": consistency,
		"preserve_published":    true,
		"publish_to_reader":     false,
	}
	created = chatPlan{
		World:  world,
		First:  first,
		Last:   last,
		Budget: budget,
		Policy: policy,
	}
	changes := map[string][2]any{
		"world":             {nil, world},
		"first_episode":     {nil, first},
		"last_episode":      {nil, last},
		"token_limit":       {nil, budget},
		"production_policy": {nil, policy},
		"instruction":       {nil, note},
	}
	return created, note, changes, nil
}

func prepareChatPatch(ctx context.Context, tx pgx.Tx, id string, before chatPlan, patch chatPatch) (chatPlan, map[string][2]any, bool, error) {
	after := before
	changes := map[string][2]any{}
	invalid := func(msg string) (chatPlan, map[string][2]any, bool, error) { return after, nil, false, errors.New(msg) }
	if patch.Revision != before.Revision {
		return invalid("计划版本已变化，请重新生成修改建议。")
	}
	if before.Stage == "drafting_plan" || before.Reserved > 0 {
		return invalid("计划有在途任务，请等待结算后再修改。")
	}
	scopeChanged := false
	switch patch.Tool {
	case "update_production_plan", "update_episode_scope", "update_token_budget", "update_production_policy":
		if patch.World != nil {
			after.World = *patch.World
		}
		if patch.First != nil {
			after.First = *patch.First
		}
		if patch.Last != nil {
			after.Last = *patch.Last
		}
		if patch.Budget != nil {
			after.Budget = *patch.Budget
		}
		if patch.KeepPublished != nil && !*patch.KeepPublished {
			return invalid("此计划工具必须保留已发布内容。")
		}
		if patch.Consistency != nil {
			if *patch.Consistency != "strict" && *patch.Consistency != "standard" {
				return invalid("人物一致性策略仅支持标准或严格。")
			}
			// Merge instead of replace so plan-level boundaries such as publish_to_reader=false survive a
			// consistency change on plans created from the workbench conversation.
			next := make(map[string]any, len(before.Policy)+2)
			for key, value := range before.Policy {
				next[key] = value
			}
			next["character_consistency"] = *patch.Consistency
			next["preserve_published"] = true
			after.Policy = next
		}
		scopeChanged = before.World != after.World || before.First != after.First || before.Last != after.Last
		var total int
		err := tx.QueryRow(ctx, q.ChatQueries["ChatEditionSize"], after.World).Scan(&total)
		if err != nil || after.First < 1 || after.Last < after.First || after.Last-after.First >= 1000 || (total == 0 && scopeChanged) || (total > 0 && after.Last > total) {
			return invalid("集数范围超出已登记的作品版本，或目录暂无总集数。")
		}
		if after.Budget < 1 || after.Budget < before.Consumed || after.Budget-before.Consumed < before.Reserved || after.Budget > 1_000_000_000 {
			return invalid("预算必须覆盖已消耗和在途预留，且不得超过 10 亿 token。")
		}
		if scopeChanged {
			var conflict bool
			if err := tx.QueryRow(ctx, q.ChatQueries["ChatRegisteredScope"], id, after.World, after.First, after.Last).Scan(&conflict); err != nil || conflict {
				return invalid("范围改动会移出已登记的分集，请另建计划。")
			}
			if after.World != before.World && before.Consumed > 0 {
				return invalid("已产生用量的计划不能更换作品，请另建计划。")
			}
		}
	case "pause_production":
		after.Paused = true
	case "resume_production":
		if before.PauseReason == "missing_usage_receipt" || before.PauseReason == "provider_exceeded_allowance" {
			return invalid("请先核对缺失或异常的模型用量回执。")
		}
		after.Paused = false
	case "retry_episode", "rebuild_episode":
		if patch.Episode < before.First || patch.Episode > before.Last || before.Paused || before.PauseReason != "" || before.Budget-before.Consumed < 16000 {
			return invalid("目前无法重排该集，请检查范围、暂停状态及剩余预算。")
		}
		var topic string
		var published bool
		var failed *bool
		if err := tx.QueryRow(ctx, q.ChatQueries["ChatEpisodeTopic"], id, patch.Episode).Scan(&topic, &published, &failed); err != nil || published {
			return invalid("找不到可重排的分集，或该集已有定稿。")
		}
		changes["episode"] = [2]any{patch.Episode, map[string]any{"topic": topic, "action": patch.Tool}}
	default:
		return invalid("该操作暂无可确认的业务接口。")
	}
	fields := []struct {
		key            string
		previous, next any
	}{
		{"world", before.World, after.World}, {"first_episode", before.First, after.First}, {"last_episode", before.Last, after.Last},
		{"token_limit", before.Budget, after.Budget}, {"production_policy", before.Policy, after.Policy}, {"operator_paused", before.Paused, after.Paused},
	}
	for _, field := range fields {
		if !reflect.DeepEqual(field.previous, field.next) {
			changes[field.key] = [2]any{field.previous, field.next}
		}
	}
	if len(changes) == 0 {
		return invalid("计划已经是所请求的状态。")
	}
	return after, changes, scopeChanged || !reflect.DeepEqual(before.Policy, after.Policy), nil
}

func resolveChat(c *gin.Context, db *pgxpool.Pool) {
	var in struct {
		Confirm *bool `binding:"required"`
	}
	id, owner := c.Param("id"), c.GetString("operator")
	if !catalog.ValidUUID(id) {
		c.Status(404)
		return
	}
	if !editorRequest(c, &in) {
		return
	}
	ctx := c.Request.Context()
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var state, world string
		var plan *string
		var proposal, storedDiff []byte
		if err := tx.QueryRow(ctx, q.ChatQueries["ChatTurnLock"], id, owner).Scan(&state, &world, &plan, &proposal, &storedDiff); err != nil {
			return err
		}
		if (state == "applied" && *in.Confirm) || (state == "rejected" && !*in.Confirm) {
			return nil
		}
		if state != "pending" {
			return errors.New("这条建议已处理或无法确认。")
		}
		if !*in.Confirm {
			_, err := tx.Exec(ctx, q.ChatQueries["ChatResolve"], id, owner, "rejected")
			return err
		}
		var patch chatPatch
		if json.Unmarshal(proposal, &patch) != nil {
			return errors.New("计划记录无效。")
		}
		if patch.Tool == "create_production_plan" {
			created, note, changes, err := prepareCreatePlanPatch(ctx, tx, world, "工作台对话新建本地验收计划", &patch)
			if err != nil {
				return err
			}
			var check, current map[string]any
			currentDiff, _ := json.Marshal(changes)
			_ = json.Unmarshal(storedDiff, &check)
			_ = json.Unmarshal(currentDiff, &current)
			if !reflect.DeepEqual(check, current) {
				return errors.New("新建计划参数已变化，请重新生成建议。")
			}
			var newPlanID string
			if err := tx.QueryRow(ctx, q.ChatQueries["ChatCreatePlan"], created.World, created.First, created.Last, created.Budget, created.Policy, note, owner).Scan(&newPlanID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, q.ChatQueries["ChatAudit"], id, newPlanID, owner, patch.Tool, 0, 1, storedDiff); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, q.ChatQueries["ChatBindTurnPlan"], id, owner, newPlanID)
			return err
		}
		if plan == nil {
			return errors.New("这条建议已处理或无法确认。")
		}
		var raw []byte
		var before chatPlan
		if err := tx.QueryRow(ctx, q.ChatQueries["ChatPlanLock"], *plan).Scan(&raw); err != nil {
			return err
		}
		if json.Unmarshal(raw, &before) != nil {
			return errors.New("计划记录无效。")
		}
		after, changes, refresh, err := prepareChatPatch(ctx, tx, *plan, before, patch)
		if err != nil {
			return err
		}
		var check map[string]any
		currentDiff, _ := json.Marshal(changes)
		var current map[string]any
		_ = json.Unmarshal(storedDiff, &check)
		_ = json.Unmarshal(currentDiff, &current)
		if !reflect.DeepEqual(check, current) {
			return errors.New("修改影响已变化，请重新生成建议。")
		}
		if patch.Tool == "retry_episode" || patch.Tool == "rebuild_episode" {
			action := changes["episode"][1].(map[string]any)
			topic := action["topic"].(string)
			var locked bool
			if err := tx.QueryRow(ctx, q.TryLockEpisode, topic).Scan(&locked); err != nil {
				return err
			}
			if !locked {
				return errors.New("分集正在验收，请稍后重新生成建议。")
			}
			var verifiedTopic string
			var published bool
			var failed *bool
			if err := tx.QueryRow(ctx, q.ChatQueries["ChatEpisodeTopic"], *plan, patch.Episode).Scan(&verifiedTopic, &published, &failed); err != nil {
				return err
			}
			if published || verifiedTopic != topic {
				return errors.New("分集已经发布或登记已变化。")
			}

			var retried int64
			if patch.Tool == "retry_episode" {
				tag, e := tx.Exec(ctx, q.ChatQueries["ChatRetryMedia"], topic)
				if e != nil {
					return e
				}
				retried = tag.RowsAffected()
			}
			if retried == 0 {
				tag, e := tx.Exec(ctx, q.ChatQueries["ChatResetResearch"], topic, patch.Tool == "retry_episode")
				if e != nil {
					return e
				}
				if tag.RowsAffected() == 0 {
					return errors.New("分集没有可重试的失败任务，或仍在执行。")
				}
			}
		}
		if _, err = tx.Exec(ctx, q.ChatQueries["ChatUpdatePlan"], *plan, after.World, after.First, after.Last, after.Budget, after.Policy, after.Paused, refresh, before.Revision); err != nil {
			return err
		}
		note := "工作台已确认 " + patch.Tool
		if patch.Consistency != nil {
			note += "。人物一致性：" + *patch.Consistency
		}
		if _, err = tx.Exec(ctx, q.ChatQueries["ChatPlanNote"], *plan, before.Revision+1, note, owner); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, q.ChatQueries["ChatAudit"], id, *plan, owner, patch.Tool, before.Revision, before.Revision+1, storedDiff); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, q.ChatQueries["ChatResolve"], id, owner, "applied")
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
