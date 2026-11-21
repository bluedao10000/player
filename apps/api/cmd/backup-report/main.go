// backup-report reconciles the backup CronJob's retained Jobs with Guangzhou.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	"github.com/jackc/pgx/v5"
)

const account = "/var/run/secrets/kubernetes.io/serviceaccount/"

type meta struct {
	Name, UID       string
	Annotations     map[string]string
	OwnerReferences []struct{ Kind, Name, UID string }
}

func (m meta) owner(kind, name string) string {
	for _, o := range m.OwnerReferences {
		if o.Kind == kind && (name == "" || name == o.Name) {
			return o.UID
		}
	}
	return ""
}

type job struct {
	Metadata meta
	Status   struct {
		StartTime  *time.Time
		Conditions []struct {
			Type, Status, Reason string
			LastTransitionTime   time.Time
		}
	}
}
type termination struct {
	ExitCode        int
	Message, Reason string
	FinishedAt      time.Time
}
type pod struct {
	Metadata meta
	Status   struct {
		ContainerStatuses []struct {
			Name  string
			State struct{ Terminated *termination }
		}
	}
}
type receipt struct {
	Bytes    int64
	Tables   int
	Verified bool
	Stage    string
}

func proof(s string) *receipt {
	var r receipt
	if json.Unmarshal([]byte(s), &r) != nil || !r.Verified || r.Bytes <= 0 || r.Tables <= 0 {
		return nil
	}
	return &r
}

type kube struct {
	client           *http.Client
	token, namespace string
}

func (k kube) read(ctx context.Context, path string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://kubernetes.default.svc"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	res, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Kubernetes HTTP %d for %s", res.StatusCode, req.URL.Path)
	}
	return res.Body, nil
}

func (k kube) list(ctx context.Context, path string, out any) error {
	b, e := k.read(ctx, path)
	if e != nil {
		return e
	}
	defer b.Close()
	return json.NewDecoder(b).Decode(out)
}

func failure(logs, condition string, t *termination) string {
	if t != nil && t.Reason == "OOMKilled" {
		return "备份容器超出内存限制"
	}
	detail := strings.ToLower(logs)
	for _, v := range [][2]string{
		{"no space left", "备份存储空间不足"},
		{"server closed the connection unexpectedly", "PostgreSQL 连接在执行中中断"},
		{"connection timed out", "连接 PostgreSQL 超时"},
		{"timeout expired", "连接 PostgreSQL 超时"},
		{"connection refused", "PostgreSQL 拒绝连接"},
		{"permission denied", "权限不足"},
		{"password authentication failed", "PostgreSQL 身份验证失败"},
		{"另一个备份正在运行", "已有备份占用卷锁"},
	} {
		if strings.Contains(detail, v[0]) {
			return v[1]
		}
	}
	if t != nil {
		var r receipt
		_ = json.Unmarshal([]byte(t.Message), &r)
		if stage := map[string]string{"export": "源库快照导出", "checksum": "归档校验和检查", "restore": "恢复到校验库", "counts": "恢复后逐表行数比对", "rotate": "切换恢复库或归档"}[r.Stage]; stage != "" {
			return stage + "失败"
		}
	}
	switch condition {
	case "DeadlineExceeded":
		return "超过 Job 执行时限"
	case "BackoffLimitExceeded":
		return "重试次数耗尽，未取得更具体的容器原因"
	}
	return "Kubernetes 已判定任务失败，未取得具体原因"
}

const save = `INSERT INTO backup_runs(job_uid,job_name,started_at,finished_at,status,archive_bytes,verified_tables,failure_reason,trigger_kind)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
 ON CONFLICT(job_uid) DO UPDATE SET finished_at=excluded.finished_at,status=excluded.status,
 archive_bytes=coalesce(excluded.archive_bytes,backup_runs.archive_bytes),
 verified_tables=coalesce(excluded.verified_tables,backup_runs.verified_tables),
 failure_reason=coalesce(backup_runs.failure_reason,excluded.failure_reason),
 trigger_kind=CASE WHEN excluded.trigger_kind='unknown' THEN backup_runs.trigger_kind ELSE excluded.trigger_kind END
 WHERE backup_runs.status='running' OR
 (backup_runs.status=excluded.status AND backup_runs.trigger_kind='unknown' AND excluded.trigger_kind<>'unknown') OR (backup_runs.status='success' AND backup_runs.archive_bytes IS NULL AND excluded.archive_bytes IS NOT NULL)`

func clusterClient() (kube, error) {
	ns, e := os.ReadFile(account + "namespace")
	if e != nil {
		return kube{}, e
	}
	token, e := os.ReadFile(account + "token")
	if e != nil {
		return kube{}, e
	}
	pem, e := os.ReadFile(account + "ca.crt")
	if e != nil {
		return kube{}, e
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return kube{}, errors.New("invalid cluster CA")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	return kube{
		client:    &http.Client{Timeout: 15 * time.Second, Transport: transport},
		token:     strings.TrimSpace(string(token)),
		namespace: strings.TrimSpace(string(ns)),
	}, nil
}

func (j job) triggerKind() string {
	if j.Metadata.Annotations["cronjob.kubernetes.io/instantiate"] == "manual" {
		return "manual"
	}
	if j.Metadata.Annotations["batch.kubernetes.io/cronjob-scheduled-timestamp"] != "" {
		return "scheduled"
	}
	return "unknown"
}

func (j job) outcome() (string, string, *time.Time) {
	for _, condition := range j.Status.Conditions {
		if condition.Status != "True" {
			continue
		}
		switch condition.Type {
		case "Complete":
			return "success", condition.Reason, &condition.LastTransitionTime
		case "Failed":
			return "failed", condition.Reason, &condition.LastTransitionTime
		}
	}
	return "running", "", nil
}

type attempt struct {
	name  string
	ended *termination
}

func lastAttempt(pods []pod, uid, state string) attempt {
	var last attempt
	for _, p := range pods {
		if p.Metadata.owner("Job", "") != uid {
			continue
		}
		for _, c := range p.Status.ContainerStatuses {
			t := c.State.Terminated
			if c.Name != "backup" || t == nil || (state == "success" && t.ExitCode != 0) {
				continue
			}
			if last.ended == nil || t.FinishedAt.After(last.ended.FinishedAt) {
				last = attempt{p.Metadata.Name, t}
			}
		}
	}
	return last
}

func (k kube) failureReason(ctx context.Context, podPath, condition string, last attempt) string {
	var logs string
	if last.name != "" {
		path := podPath + "/" + url.PathEscape(last.name) + "/log?container=backup&tailLines=50&limitBytes=8192"
		body, err := k.read(ctx, path)
		if err == nil {
			defer body.Close()
			tail, _ := io.ReadAll(io.LimitReader(body, 8192))
			logs = string(tail)
		}
	}
	return failure(logs, condition, last.ended)
}

// Retain a failed attempt even if Kubernetes later retries the Job successfully.
func (k kube) saveFailedAttempts(ctx context.Context, tx pgx.Tx, podPath string, pods []pod, jobUID string) error {
	for _, p := range pods {
		if p.Metadata.owner("Job", "") != jobUID {
			continue
		}
		for _, c := range p.Status.ContainerStatuses {
			t := c.State.Terminated
			if c.Name != "backup" || t == nil || t.ExitCode == 0 {
				continue
			}
			var recorded bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM backup_failed_attempts WHERE pod_uid=$1)", p.Metadata.UID).Scan(&recorded); err != nil {
				return err
			}
			if recorded {
				continue
			}
			reason := k.failureReason(ctx, podPath, "", attempt{p.Metadata.Name, t})
			_, err := tx.Exec(ctx, `INSERT INTO backup_failed_attempts(pod_uid,job_uid,finished_at,reason)
                VALUES($1,$2,$3,$4) ON CONFLICT(pod_uid) DO NOTHING`, p.Metadata.UID, jobUID, t.FinishedAt, reason)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func run(ctx context.Context) error {
	k, e := clusterClient()
	if e != nil {
		return e
	}
	var jobs struct{ Items []job }
	var pods struct{ Items []pod }
	if e = k.list(ctx, "/apis/batch/v1/namespaces/"+k.namespace+"/jobs", &jobs); e != nil {
		return e
	}
	path := "/api/v1/namespaces/" + k.namespace + "/pods"
	if e = k.list(ctx, path+"?labelSelector=app%3Dfolio-backup-job", &pods); e != nil {
		return e
	}
	db, e := pgx.Connect(ctx, "host=/source dbname=folio user=folio_backup_report sslmode=disable connect_timeout=12")
	if e != nil {
		return e
	}
	defer db.Close(ctx)
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	for _, j := range jobs.Items {
		if j.Metadata.owner("CronJob", "guangzhou-backup") == "" || j.Status.StartTime == nil {
			continue
		}
		state, condition, end := j.outcome()
		last := lastAttempt(pods.Items, j.Metadata.UID, state)
		var size, tables, reason any
		switch state {
		case "success":
			var r *receipt
			if last.ended != nil {
				r = proof(last.ended.Message)
			}
			// Older Jobs can be backfilled after rechecking the named archive on disk.
			if r == nil {
				r = proof(j.Metadata.Annotations["folio.codeiy.com/verified-archive"])
			}
			if r != nil {
				size = r.Bytes
				tables = r.Tables
			}
		case "failed":
			reason = k.failureReason(ctx, path, condition, last)
		}
		if _, e = tx.Exec(ctx, save, j.Metadata.UID, j.Metadata.Name, j.Status.StartTime, end, state, size, tables, reason, j.triggerKind()); e != nil {
			return e
		}
		if e = k.saveFailedAttempts(ctx, tx, path, pods.Items, j.Metadata.UID); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO backup_observer(observed_at) VALUES(now()) ON CONFLICT(singleton) DO UPDATE SET observed_at=excluded.observed_at`)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if e := run(ctx); e != nil {
		log.Fatal(e)
	}
}
