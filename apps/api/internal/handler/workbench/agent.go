package workbench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/database"
	q "anime-agent/api/internal/repository/queries/workbench"
	"anime-agent/api/internal/service/modelusage"
)

var errAgentCancelled = errors.New("workbench run cancelled")

var agentHTTP = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type agentMention struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type agentRequest struct {
	Intent         string  `json:"intent"`
	Model          string  `json:"model"`
	ConversationID *string `json:"conversation_id"`
	Context        struct {
		Page     string         `json:"page"`
		Scope    string         `json:"scope"`
		Mentions []agentMention `json:"mentions"`
	} `json:"context"`
}

type workFacts struct {
	Work       json.RawMessage   `json:"work"`
	Gaps       []json.RawMessage `json:"gaps"`
	Dimensions json.RawMessage   `json:"dimensions"`
}

type agentInput struct {
	Intent      string            `json:"intent"`
	Page        string            `json:"page"`
	Scope       string            `json:"scope,omitempty"`
	History     json.RawMessage   `json:"history"`
	Works       []workFacts       `json:"works"`
	Records     []json.RawMessage `json:"records,omitempty"`
	PlanWorkID  string            `json:"plan_work_id,omitempty"`
	BriefingDay string            `json:"briefing_day,omitempty"`

	runRefs  map[string]bool
	briefing json.RawMessage
}

type artifactRequest struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

type plannedTask struct {
	Title    string `json:"title"`
	Priority string `json:"priority"`
}

type proposedPlan struct {
	WorkID    string        `json:"work_id"`
	Title     string        `json:"title"`
	Objective string        `json:"objective"`
	Tasks     []plannedTask `json:"tasks"`
}

type upstreamEvent struct {
	Type         string            `json:"type"`
	Text         string            `json:"t"`
	Plan         *proposedPlan     `json:"plan"`
	Artifact     *artifactRequest  `json:"artifact"`
	InputTokens  *int64            `json:"input_tokens"`
	OutputTokens *int64            `json:"output_tokens"`
	Receipt      *modelusage.Event `json:"receipt"`
}

type agentStream struct {
	c        *gin.Context
	handler  handler
	id       string
	sequence int
}

func (s *agentStream) send(event gin.H) error {
	if err := writeFrame(s.c.Writer, event); err != nil {
		return err
	}
	s.c.Writer.Flush()
	return nil
}

func writeFrame(w io.Writer, event gin.H) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", body)
	return err
}

func (s *agentStream) emit(ctx context.Context, event gin.H) error {
	// Bound backpressure on PostgreSQL while preserving every delivered token.
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := s.handler.db.Exec(save, q.Write["RecordEvent"], s.id, s.sequence+1, event)
	if err != nil {
		return err
	}
	s.sequence++
	return s.send(event)
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func (h handler) agentRun(c *gin.Context) {
	var request agentRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if c.ShouldBindJSON(&request) != nil {
		apiFailure(c, invalid("body"))
		return
	}
	request.Intent = strings.TrimSpace(request.Intent)
	switch {
	case request.Intent == "" || len(request.Intent) > 8000:
		apiFailure(c, invalid("intent"))
		return
	case !oneOf(request.Model, "", "folio-agent"):
		apiFailure(c, invalid("model"))
		return
	case request.ConversationID != nil && !catalog.ValidUUID(*request.ConversationID):
		apiFailure(c, invalid("conversation_id"))
		return
	case !oneOf(request.Context.Page, "", "today", "inbox", "knowledge", "activity", "chat"):
		apiFailure(c, invalid("context.page"))
		return
	case len(request.Context.Scope) > 80:
		apiFailure(c, invalid("context.scope"))
		return
	case len(request.Context.Mentions) > 8:
		apiFailure(c, invalid("context.mentions"))
		return
	}
	for _, mention := range request.Context.Mentions {
		if mention.ID == "" || len(mention.ID) > 256 {
			apiFailure(c, invalid("context.mentions"))
			return
		}
	}
	base := os.Getenv("KNOWLEDGE_RESEARCH_URL")
	endpoint, err := url.Parse(base)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || !oneOf(endpoint.Scheme, "http", "https") {
		apiFailure(c, &apiError{503, "upstream_unavailable", "Agent 服务尚不可用。", nil})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Minute)
	defer cancel()
	setup, stopSetup := context.WithTimeout(ctx, 10*time.Second)
	input, conversation, runID, err := h.prepareAgent(setup, request, c.GetString("operator"))
	stopSetup()
	if err != nil {
		apiFailure(c, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	stream := agentStream{handler: h, c: c, id: runID, sequence: 2}
	start := gin.H{"type": "run_started", "run_id": runID, "conversation_id": conversation}
	var result agentResult
	if err = stream.send(start); err == nil {
		result, err = stream.infer(ctx, endpoint.JoinPath("internal", "workbench", "runs", runID).String(), input)
	}
	cancel()
	state, message := "completed", ""
	if c.Request.Context().Err() != nil || errors.Is(err, errAgentCancelled) {
		state, message = "cancelled", "本次生成已停止。"
	} else if err != nil {
		state, message = "failed", "Agent 暂时无法完成本次请求。"
	}
	if state == "failed" {
		log.Printf("workbench agent run %s: %v", runID, err)
	}
	stream.finish(state, message, input, result)
}

func (h handler) prepareAgent(ctx context.Context, request agentRequest, operator string) (agentInput, string, string, error) {
	input := agentInput{Intent: request.Intent, Page: request.Context.Page, Scope: request.Context.Scope, History: json.RawMessage("[]"), Works: []workFacts{}, runRefs: make(map[string]bool)}
	workIDs := make(map[string]bool)
	selected := []string{}
	var conversation, runID string
	err := pgx.BeginFunc(ctx, h.db, func(tx pgx.Tx) error {
		if request.ConversationID != nil {
			if err := tx.QueryRow(ctx, q.Read["Conversation"], *request.ConversationID, operator).Scan(&conversation); err != nil {
				return err
			}
			history, err := database.ReadJSON(ctx, tx, q.Read["ConversationHistory"], conversation)
			if err != nil {
				return err
			}
			input.History = history
		} else if err := tx.QueryRow(ctx, q.Write["CreateConversation"], operator).Scan(&conversation); err != nil {
			return err
		}
		for _, mention := range request.Context.Mentions {
			switch mention.Kind {
			case "work":
				if !catalog.ValidUUID(mention.ID) {
					return invalid("context.mentions")
				}
				id := strings.ToLower(mention.ID)
				if !workIDs[id] {
					workIDs[id] = true
					selected = append(selected, id)
				}
			case "decision", "run":
				name := "Run"
				if mention.Kind == "decision" {
					if !catalog.ValidUUID(mention.ID) {
						return invalid("context.mentions")
					}
					name = "Decision"
				}
				body, err := database.ReadJSON(ctx, tx, q.Read[name], mention.ID)
				if err != nil {
					return err
				}
				input.Records = append(input.Records, body)
				if name == "Run" {
					input.runRefs[mention.ID] = true
				}
			}
		}
		if len(selected) == 0 {
			found, err := database.ReadJSON(ctx, tx, q.Read["FindMentionedWorks"], request.Intent)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(found, &selected); err != nil {
				return err
			}
		}
		if len(selected) > 3 {
			return invalid("context.mentions")
		}
		var primary string
		for _, id := range selected {
			body, err := database.ReadJSON(ctx, tx, q.Read["Work"], id)
			if err != nil {
				return err
			}
			gaps, err := database.ReadJSON(ctx, tx, q.Read["AgentWorkGaps"], id)
			if err != nil {
				return err
			}
			var parsed []json.RawMessage
			if err = json.Unmarshal(gaps, &parsed); err != nil {
				return err
			}
			dimensions, err := database.ReadJSON(ctx, tx, q.Read["ArtifactWorkDimensions"], id)
			if err != nil {
				return err
			}
			input.Works = append(input.Works, workFacts{Work: body, Gaps: parsed, Dimensions: dimensions})
			var work struct {
				Status string `json:"publication_status"`
			}
			if err = json.Unmarshal(body, &work); err != nil {
				return err
			}
			if primary == "" {
				primary = id
			}
			if len(selected) == 1 && work.Status != "archived" {
				input.PlanWorkID = id
			}
		}
		if request.Context.Page == "today" {
			now := time.Now().In(beijing)
			input.BriefingDay = now.Format(time.DateOnly)
			start, _ := time.ParseInLocation(time.DateOnly, input.BriefingDay, beijing)
			body, err := database.ReadJSON(ctx, tx, q.Read["Briefing"], start, now, true)
			if err != nil {
				return err
			}
			input.briefing = body
			input.Records = append(input.Records, body)
		}
		if err := tx.QueryRow(ctx, q.Write["StartRun"], conversation, primary, shorten(request.Intent, 100)).Scan(&runID); err != nil {
			return err
		}
		events := []gin.H{
			{"type": "request", "intent": request.Intent, "work_ids": selected},
			{"type": "run_started", "run_id": runID, "conversation_id": conversation},
		}
		for i, event := range events {
			if _, err := tx.Exec(ctx, q.Write["RecordEvent"], runID, i+1, event); err != nil {
				return err
			}
		}
		return nil
	})
	return input, conversation, runID, err
}

type agentResult struct {
	text      strings.Builder
	plan      *proposedPlan
	artifacts []artifactRequest
	input     *int64
	output    *int64
	toolCount int
	receipt   *modelusage.Event
}

func (s *agentStream) infer(ctx context.Context, endpoint string, input agentInput) (agentResult, error) {
	var result agentResult
	if err := s.emit(ctx, gin.H{"type": "thinking_step", "id": "context", "label": "已读取工作台上下文"}); err != nil {
		return result, err
	}
	if len(input.Works) > 0 || len(input.Records) > 0 {
		if err := s.emit(ctx, gin.H{"type": "tool_call", "id": "context", "tool": "workbench.context", "label": "查询工作台事实", "status": "done"}); err != nil {
			return result, err
		}
		result.toolCount++
	}
	body, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	events := make(chan upstreamEvent)
	failure := make(chan error, 1)
	go readAgentEvents(req, events, failure)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	cancelPoll := time.NewTicker(500 * time.Millisecond)
	defer cancelPoll.Stop()
	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-cancelPoll.C:
			var cancelled bool
			if err := s.handler.db.QueryRow(ctx, q.Read["CancelRequested"], s.id).Scan(&cancelled); err != nil {
				return result, err
			}
			if cancelled {
				return result, errAgentCancelled
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(s.c.Writer, ": keep-alive\n\n"); err != nil {
				return result, err
			}
			s.c.Writer.Flush()
		case event, ok := <-events:
			if !ok {
				return result, <-failure
			}
			switch event.Type {
			case "token":
				if event.Text == "" {
					continue
				}
				if result.text.Len()+len(event.Text) > 48<<10 {
					return result, errors.New("agent response exceeds limit")
				}
				result.text.WriteString(event.Text)
				if err := s.emit(ctx, gin.H{"type": "token", "t": event.Text}); err != nil {
					return result, err
				}
			case "plan":
				result.plan = event.Plan
			case "artifact":
				if event.Artifact != nil && len(result.artifacts) < 4 {
					result.artifacts = append(result.artifacts, *event.Artifact)
				}
			case "done":
				result.receipt = event.Receipt
				result.input, result.output = nonnegative(event.InputTokens), nonnegative(event.OutputTokens)
				return result, nil
			case "error":
				return result, errors.New("model failed to complete")
			}
		}
	}
}

func readAgentEvents(req *http.Request, events chan<- upstreamEvent, failure chan<- error) {
	defer close(events)
	resp, err := agentHTTP.Do(req)
	if err != nil {
		failure <- err
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		failure <- fmt.Errorf("agent HTTP %d", resp.StatusCode)
		return
	}
	lines := bufio.NewScanner(io.LimitReader(resp.Body, 256<<10))
	lines.Buffer(make([]byte, 4096), 64<<10)
	for lines.Scan() {
		var event upstreamEvent
		if err := json.Unmarshal(lines.Bytes(), &event); err != nil {
			failure <- err
			return
		}
		select {
		case events <- event:
		case <-req.Context().Done():
			failure <- req.Context().Err()
			return
		}
		if oneOf(event.Type, "done", "error") {
			failure <- nil
			return
		}
	}
	if err = lines.Err(); err == nil {
		err = io.ErrUnexpectedEOF
	}
	failure <- err
}

func nonnegative(value *int64) *int64 {
	if value != nil && *value >= 0 {
		return value
	}
	return nil
}

func (h handler) cancelAgentRun(c *gin.Context) {
	id, ok := uuidParam(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	operator := c.GetString("operator")
	if _, err := h.db.Exec(ctx, q.Write["RequestCancel"], id, operator); err != nil {
		apiFailure(c, err)
		return
	}
	value, err := database.ReadJSON(ctx, h.db, q.Read["AgentRun"], id, operator)
	respond(c, value, err)
}
