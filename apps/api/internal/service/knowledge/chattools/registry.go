// Package chattools owns the workbench read allowlist and its exported contract.
package chattools

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	model "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/catalog"
	q "anime-agent/api/internal/repository/queries/knowledge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxSelectedWorks = 1
	MaxCalls         = 3
	ReadTimeout      = 3 * time.Second
	MaxResultBytes   = 24 * 1024
	MaxBatchBytes    = 36 * 1024

	public   = "public_catalog"
	snapshot = "current_snapshot"
	selected = "selected_workbench"
)

type Field struct {
	Type    string `json:"type"`
	Min     int    `json:"minLength"`
	Max     int    `json:"maxLength"`
	Pattern string `json:"pattern,omitempty"`
}

type Schema struct {
	Type                 string           `json:"type"`
	Properties           map[string]Field `json:"properties"`
	Required             []string         `json:"required"`
	AdditionalProperties bool             `json:"additionalProperties"`
}

type Limits struct {
	Rows        int `json:"rows,omitempty"`
	GapsPerRule int `json:"gapsPerRule,omitempty"`
	Titles      int `json:"titles,omitempty"`
	Tags        int `json:"tags,omitempty"`
	Related     int `json:"related,omitempty"`
	Bytes       int `json:"bytes,omitempty"`
}

type Spec struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	InputVersion    int     `json:"inputVersion"`
	ResponseVersion int     `json:"responseVersion"`
	Scope           string  `json:"scope"`
	Mode            string  `json:"mode"`
	Schema          *Schema `json:"schema,omitempty"`
	Limits          Limits  `json:"limits"`
}

type sections map[string]json.RawMessage

type entry struct {
	spec   Spec
	inject func(sections) any
	call   func(context.Context, *pgxpool.Pool, *Selection, string, Limits) (any, error)
}

func fromSection(key string) func(sections) any {
	return func(s sections) any { return s[key] }
}

func injected(name, description string, limits Limits, read func(sections) any) entry {
	return entry{spec: Spec{Name: name, Description: description, InputVersion: 1, ResponseVersion: 1, Scope: snapshot, Mode: "snapshot", Limits: limits}, inject: read}
}

func callable(name, description, scope, key string, field Field, limits Limits, read func(context.Context, *pgxpool.Pool, *Selection, string, Limits) (any, error)) entry {
	limits.Bytes = MaxResultBytes
	schema := &Schema{Type: "object", Properties: map[string]Field{key: field}, Required: []string{key}}
	return entry{spec: Spec{Name: name, Description: description, InputVersion: 1, ResponseVersion: 1, Scope: scope, Mode: "callable", Schema: schema, Limits: limits}, call: read}
}

// The snapshot projection is version 1. Its row limits are applied by ChatContext.
var snapshotEntries = []entry{
	injected("get_workspace_context", "当前作品资料、盘点、所选审稿任务和治理摘要，含最多 5 份在途计划。", Limits{Rows: 5}, func(s sections) any {
		return sections{"work": s["work"], "inventory": s["inventory"], "editor_task": s["editor_task"], "governance": s["governance"]}
	}),
	injected("get_knowledge_completeness", "当前作品的七维完整度，每项规则最多 6 条缺项。", Limits{GapsPerRule: 6}, fromSection("completeness")),
	injected("get_production_plan", "当前作品所选制作计划。", Limits{Rows: 1}, fromSection("plan")),
	injected("get_episode_status", "当前作品的分集制作、验收和发布状态，最多 12 条。", Limits{Rows: 12}, fromSection("episodes")),
	injected("get_media_candidates", "当前作品的候选图记录，最多 12 条。", Limits{Rows: 12}, fromSection("media")),
	injected("get_execution_history", "当前作品的取证运行历史，最多 12 条。", Limits{Rows: 12}, fromSection("history")),
}

var registry = append(slices.Clone(snapshotEntries),
	callable("search_anime_works", "按原名、译名或别名搜索公开目录，最多 8 部作品，返回作品 UUID。", public, "q", Field{Type: "string", Min: 1, Max: 128}, Limits{Rows: 8}, searchWorks),
	callable("read_anime_work", "按作品 UUID 读取公开目录的基础资料、关联作品和来源版本摘要（provenance），不含来源平台 ID 或 URL 清单。", public, "id", Field{Type: "string", Min: 36, Max: 36, Pattern: "^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$"}, Limits{Rows: 1, Titles: 20, Tags: 30, Related: 12}, readWork),
	callable("read_selected_work", "读取用户为本轮选定的另一部作品，返回带作品标识的工作台快照。每类分集、候选图和取证运行最多 12 条。", selected, "work", Field{Type: "string", Min: 1, Max: 128}, Limits{Rows: 12, GapsPerRule: 6}, readSelected),
)

func Manifest() []Spec {
	specs := make([]Spec, 0, len(registry))
	for _, item := range registry {
		specs = append(specs, item.spec)
	}
	return specs
}

func Capabilities(hasSelection bool) []string {
	names := make([]string, 0, len(registry))
	for _, item := range registry {
		if item.spec.Scope != selected || hasSelection {
			names = append(names, item.spec.Name)
		}
	}
	return names
}

func Snapshot(raw json.RawMessage) (map[string]any, []string, error) {
	var data sections
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, nil, err
	}
	facts := make(map[string]any)
	names := make([]string, 0, len(snapshotEntries))
	for _, item := range snapshotEntries {
		facts[item.spec.Name] = item.inject(data)
		names = append(names, item.spec.Name)
	}
	return facts, names, nil
}

type Call struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Result struct {
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func (item entry) execute(ctx context.Context, db *pgxpool.Pool, selection *Selection, raw json.RawMessage) (any, error) {
	if item.spec.Scope == selected && selection == nil {
		return nil, errNotSelected
	}
	key := item.spec.Schema.Required[0]
	field := item.spec.Schema.Properties[key]
	var args map[string]json.RawMessage
	var value string
	if json.Unmarshal(raw, &args) != nil || len(args) != 1 || json.Unmarshal(args[key], &value) != nil ||
		strings.TrimSpace(value) == "" || len(utf16.Encode([]rune(value))) > field.Max ||
		(field.Pattern != "" && !model.ValidUUID(value)) {
		return nil, ErrArguments
	}
	return item.call(ctx, db, selection, value, item.spec.Limits)
}

var (
	ErrArguments   = errors.New("invalid_arguments")
	errNotSelected = errors.New("work_not_selected")
	ErrScope       = errors.New("work_scope_mismatch")
	ErrPayload     = errors.New("payload_limit")
)

func resultError(err error) string {
	switch {
	case errors.Is(err, ErrArguments), errors.Is(err, errNotSelected), errors.Is(err, ErrScope), errors.Is(err, ErrPayload):
		return err.Error()
	case errors.Is(err, pgx.ErrNoRows):
		return "not_found"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "unavailable"
	}
}

// Execute handles one batch. The read deadline and byte budget are shared by all calls.
func Execute(ctx context.Context, db *pgxpool.Pool, selection *Selection, calls []Call) ([]Result, []string) {
	ctx, cancel := context.WithTimeout(ctx, ReadTimeout)
	defer cancel()
	results := make([]Result, 0, min(len(calls), MaxCalls))
	var names []string
	if len(calls) > MaxCalls {
		return []Result{{Error: "call_limit"}}, nil
	}
	used := 0
	for _, call := range calls {
		result := Result{Name: call.Name, Error: "tool_not_available"}
		for _, item := range registry {
			if item.spec.Name != call.Name || item.call == nil {
				continue
			}
			result.Error = ""
			data, err := item.execute(ctx, db, selection, call.Arguments)
			if err == nil {
				result.Data, err = json.Marshal(data)
			}
			if err == nil && (len(result.Data) > item.spec.Limits.Bytes || used+len(result.Data) > MaxBatchBytes) {
				err = ErrPayload
			}
			if err != nil {
				result.Data, result.Error = nil, resultError(err)
			} else {
				used += len(result.Data)
				names = append(names, item.spec.Name)
			}
			break
		}
		// An unknown model-supplied name can be arbitrarily long.
		if result.Error == "tool_not_available" {
			result.Name = ""
		}
		results = append(results, result)
	}
	return results, names
}

func searchWorks(ctx context.Context, db *pgxpool.Pool, _ *Selection, query string, limits Limits) (any, error) {
	raw, err := catalog.Works(ctx, db, query, "", limits.Rows)
	if err != nil {
		return nil, err
	}
	var page struct{ Items json.RawMessage }
	err = json.Unmarshal(raw, &page)
	return page.Items, err
}

// Work has the full public catalog projection. Retain the established Agent subset.
func readWork(ctx context.Context, db *pgxpool.Pool, _ *Selection, id string, limits Limits) (any, error) {
	raw, err := catalog.Work(ctx, db, id)
	if err != nil {
		return nil, err
	}
	var work sections
	if err = json.Unmarshal(raw, &work); err != nil {
		return nil, err
	}
	detail := sections{}
	for _, key := range []string{"id", "canonical_title", "type", "season_year", "episodes", "status", "studios", "provenance"} {
		detail[key] = work[key]
	}
	for _, list := range []struct {
		from, to string
		count    int
	}{
		{"titles", "titles", limits.Titles}, {"tags", "tags", limits.Tags}, {"relations", "related", limits.Related},
	} {
		var values []json.RawMessage
		if err = json.Unmarshal(work[list.from], &values); err != nil {
			return nil, err
		}
		detail[list.to], err = json.Marshal(values[:min(len(values), list.count)])
		if err != nil {
			return nil, err
		}
	}
	return detail, nil
}

type Selection struct {
	Work    string  `json:"work"`
	Plan    *string `json:"plan_id,omitempty"`
	Task    *string `json:"task_id,omitempty"`
	Episode int     `json:"episode,omitempty"`
	UUID    string  `json:"-"`
	Slug    string  `json:"-"`
	Title   string  `json:"-"`
}

func Resolve(ctx context.Context, db *pgxpool.Pool, current string, choice Selection) (*Selection, error) {
	choice.Work = strings.TrimSpace(choice.Work)
	if choice.Work == "" || len(choice.Work) > 128 || choice.Episode < 0 || choice.Episode > 100000 ||
		(choice.Plan != nil && !model.ValidUUID(*choice.Plan)) || (choice.Task != nil && !model.ValidUUID(*choice.Task)) {
		return nil, ErrArguments
	}
	if model.ValidUUID(choice.Work) {
		choice.Work = strings.ToLower(choice.Work)
	}
	var valid bool
	err := db.QueryRow(ctx, q.ChatQueries["ChatSelection"], choice.Work, choice.Plan, choice.Task, choice.Episode).
		Scan(&choice.UUID, &choice.Slug, &choice.Title, &valid)
	if err != nil {
		return nil, err
	}
	if !valid || choice.Slug == current {
		return nil, ErrScope
	}
	return &choice, nil
}

func readSelected(ctx context.Context, db *pgxpool.Pool, selection *Selection, work string, _ Limits) (any, error) {
	if work != selection.Slug && !strings.EqualFold(work, selection.UUID) {
		return nil, errNotSelected
	}
	if _, err := Resolve(ctx, db, "", *selection); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	err := db.QueryRow(ctx, q.ChatQueries["ChatContext"], selection.Slug, selection.Plan, selection.Episode, selection.Task, selection.UUID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	// Match the prefetched authorization against the returned data as well.
	var data sections
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if (selection.Plan != nil && string(data["plan"]) == "null") ||
		(selection.Task != nil && string(data["editor_task"]) == "null") {
		return nil, ErrScope
	}
	facts, _, err := Snapshot(raw)
	return map[string]any{"work": map[string]string{"id": selection.UUID, "slug": selection.Slug, "title": selection.Title}, "tools": facts}, err
}
