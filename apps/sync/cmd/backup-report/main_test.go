package main

import (
	"encoding/json"
	"testing"
)

func TestVerifiedReceipt(t *testing.T) {
	for _, s := range []string{"", "{}", `{"verified":false,"tables":92,"bytes":123}`, `{"verified":true,"tables":0,"bytes":123}`} {
		if proof(s) != nil {
			t.Fatal("accepted incomplete verification", s)
		}
	}
	r := proof(`{"verified":true,"tables":92,"bytes":46787651}`)
	if r == nil || r.Bytes != 46787651 || r.Tables != 92 {
		t.Fatal(r)
	}
}

func TestFailedJobWithoutCompletionTime(t *testing.T) {
	var j job
	if err := json.Unmarshal([]byte(`{"status":{"conditions":[{"type":"FailureTarget","status":"True"},{"type":"Failed","status":"True","reason":"BackoffLimitExceeded","lastTransitionTime":"2026-11-21T04:47:52Z"}]}}`), &j); err != nil {
		t.Fatal(err)
	}
	state, reason, end := j.outcome()
	if state != "failed" || reason != "BackoffLimitExceeded" || end == nil || end.Second() != 52 {
		t.Fatal(state, reason, end)
	}
	logs := "pg_dump: error: server closed the connection unexpectedly\nERROR: division by zero"
	if got := failure(logs, reason, nil); got != "PostgreSQL 连接在执行中中断" {
		t.Fatal(got)
	}
	if got := failure("unrecognized diagnostic containing private data", reason, &termination{Message: `{"stage":"counts"}`}); got != "恢复后逐表行数比对失败" {
		t.Fatal(got)
	}
}
