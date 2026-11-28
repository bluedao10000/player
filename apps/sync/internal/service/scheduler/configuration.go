package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	q "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/service/modelusage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type configurationJob struct {
	Request, Call, World, Brief string
	Revision, Number            int
	Previous                    json.RawMessage
}

func prepareConfiguration(ctx context.Context, db *pgxpool.Pool, agent string) error {
	var job configurationJob
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, q.Configuration["Expire"]); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, q.Configuration["Next"]).Scan(&job.Request, &job.Revision, &job.World, &job.Number, &job.Brief, &job.Previous)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		call := modelusage.NewID()
		if len(job.Previous) == 0 {
			if _, err = tx.Exec(ctx, q.Configuration["Start"], call, job.Request, job.Revision, job.Number); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, q.Configuration["Finish"], call, "缺少可复用的同版来源配置", false); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, q.Configuration["Stage"], job.Request, job.Revision, "blocked")
			return err
		}
		var ok bool
		if err = tx.QueryRow(ctx, q.Configuration["Reserve"], job.Request, call).Scan(&ok); err != nil || !ok {
			return err
		}
		job.Call = call
		_, err = tx.Exec(ctx, q.Configuration["Start"], call, job.Request, job.Revision, job.Number)
		return err
	})
	if err != nil || job.Call == "" {
		return err
	}

	result, usage, reason := generateConfiguration(ctx, agent, job)
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	return pgx.BeginFunc(finishCtx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(finishCtx, q.SettlePlanDraft, job.Request, job.Call, usage); err != nil {
			return err
		}
		if reason == "" && len(result.Outline) > 0 {
			tag, err := tx.Exec(finishCtx, q.Configuration["SaveOutline"], job.Request, job.Revision, job.Number, result.Outline)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				reason = "提纲保存前制作计划已变化"
			}
		} else if reason == "" {
			err := pgx.BeginFunc(finishCtx, tx, func(registration pgx.Tx) error {
				return knowledge.RegisterEpisodeConfiguration(finishCtx, registration, job.Request, job.Revision, job.Number, job.Previous, result)
			})
			if err != nil {
				reason = err.Error()
			}
		}
		var recordedError *string
		if reason != "" {
			recordedError = &reason
		}
		tag, err := tx.Exec(finishCtx, q.Configuration["Finish"], job.Call, recordedError, reason == "" && len(result.Outline) == 0)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("配置租约已过期")
		}
		if reason == "" {
			return nil
		}
		var attempts int
		if err = tx.QueryRow(finishCtx, q.Configuration["Attempts"], job.Request, job.Revision, job.Number).Scan(&attempts); err != nil {
			return err
		}
		if usage == nil || attempts >= 2 || (usage != nil && *usage == 0) {
			_, err = tx.Exec(finishCtx, q.Configuration["Stage"], job.Request, job.Revision, "blocked")
		}
		return err
	})
}

func generateConfiguration(ctx context.Context, agent string, job configurationJob) (knowledge.EpisodeConfiguration, *int64, string) {
	var result struct {
		knowledge.EpisodeConfiguration
		Tokens *int64 `json:"tokens"`
		Error  string `json:"error"`
	}
	input, err := json.Marshal(map[string]any{"previous": job.Previous, "number": job.Number, "brief": job.Brief})
	if err != nil {
		return result.EpisodeConfiguration, nil, "无法构造分集配置请求"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(agent, "/")+"/internal/episode-configuration/"+job.Call, bytes.NewReader(input))
	if err != nil {
		return result.EpisodeConfiguration, nil, "分集配置接口地址不可用"
	}
	client := http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return result.EpisodeConfiguration, nil, "分集配置调用中断，等待用量对账"
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&result) != nil ||
		result.Tokens == nil || *result.Tokens < 0 {
		return result.EpisodeConfiguration, nil, "分集配置调用缺少可核对的用量回执"
	}
	if result.Error == "" && (*result.Tokens == 0 || (len(result.Spec) == 0) == (len(result.Outline) == 0)) {
		return result.EpisodeConfiguration, result.Tokens, "分集配置结果不完整"
	}
	return result.EpisodeConfiguration, result.Tokens, result.Error
}
