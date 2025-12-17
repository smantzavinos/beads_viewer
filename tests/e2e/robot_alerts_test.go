package main_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestRobotAlerts_BasicAndFilters(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	now := time.Now().UTC()
	staleUpdated := now.AddDate(0, 0, -20).Format(time.RFC3339) // warning (default 14d)
	staleCreated := now.AddDate(0, 0, -25).Format(time.RFC3339) // keep valid ordering
	freshTime := now.AddDate(0, 0, -1).Format(time.RFC3339)

	// ROOT unblocks 3 issues => blocking_cascade (info); STALE triggers stale_issue (warning).
	writeBeads(t, env, fmt.Sprintf(
		`{"id":"ROOT","title":"Root","status":"open","priority":1,"issue_type":"task","created_at":"%s","updated_at":"%s"}
{"id":"D1","title":"Dep1","status":"open","priority":2,"issue_type":"task","created_at":"%s","updated_at":"%s","dependencies":[{"issue_id":"D1","depends_on_id":"ROOT","type":"blocks"}]}
{"id":"D2","title":"Dep2","status":"open","priority":2,"issue_type":"task","created_at":"%s","updated_at":"%s","dependencies":[{"issue_id":"D2","depends_on_id":"ROOT","type":"blocks"}]}
{"id":"D3","title":"Dep3","status":"open","priority":2,"issue_type":"task","created_at":"%s","updated_at":"%s","dependencies":[{"issue_id":"D3","depends_on_id":"ROOT","type":"blocks"}]}
{"id":"STALE","title":"Stale issue","status":"open","priority":3,"issue_type":"task","created_at":"%s","updated_at":"%s"}`,
		freshTime, freshTime,
		freshTime, freshTime,
		freshTime, freshTime,
		freshTime, freshTime,
		staleCreated, staleUpdated,
	))

	type alert struct {
		Type     string `json:"type"`
		Severity string `json:"severity"`
		IssueID  string `json:"issue_id"`
	}
	type payload struct {
		DataHash string  `json:"data_hash"`
		Alerts   []alert `json:"alerts"`
		Summary  struct {
			Total    int `json:"total"`
			Critical int `json:"critical"`
			Warning  int `json:"warning"`
			Info     int `json:"info"`
		} `json:"summary"`
	}

	run := func(args ...string) payload {
		t.Helper()
		cmd := exec.Command(bv, args...)
		cmd.Dir = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		var p payload
		if err := json.Unmarshal(out, &p); err != nil {
			t.Fatalf("json decode: %v\nout=%s", err, out)
		}
		return p
	}

	// Unfiltered output should include at least one stale and one cascade alert.
	base := run("--robot-alerts")
	if base.DataHash == "" {
		t.Fatalf("missing data_hash")
	}
	if base.Summary.Total != len(base.Alerts) {
		t.Fatalf("summary.total=%d; want %d", base.Summary.Total, len(base.Alerts))
	}
	foundStale := false
	foundCascade := false
	for _, a := range base.Alerts {
		if a.Type == "stale_issue" && a.Severity == "warning" && a.IssueID == "STALE" {
			foundStale = true
		}
		if a.Type == "blocking_cascade" && a.IssueID == "ROOT" {
			foundCascade = true
		}
	}
	if !foundStale {
		t.Fatalf("expected stale_issue warning for STALE, got %+v", base.Alerts)
	}
	if !foundCascade {
		t.Fatalf("expected blocking_cascade for ROOT, got %+v", base.Alerts)
	}

	// Type filter.
	onlyStale := run("--robot-alerts", "--alert-type=stale_issue")
	if len(onlyStale.Alerts) == 0 {
		t.Fatalf("expected stale_issue alerts, got 0")
	}
	for _, a := range onlyStale.Alerts {
		if a.Type != "stale_issue" {
			t.Fatalf("unexpected alert type %q in filtered output: %+v", a.Type, a)
		}
	}

	// Severity filter.
	onlyWarning := run("--robot-alerts", "--severity=warning")
	if len(onlyWarning.Alerts) == 0 {
		t.Fatalf("expected warning alerts, got 0")
	}
	for _, a := range onlyWarning.Alerts {
		if a.Severity != "warning" {
			t.Fatalf("unexpected severity %q in filtered output: %+v", a.Severity, a)
		}
	}
}
