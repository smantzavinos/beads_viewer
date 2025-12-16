package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestEstimateETAForIssue_Basic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "Test issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	if eta.IssueID != "test-1" {
		t.Errorf("Expected issue ID 'test-1', got %q", eta.IssueID)
	}

	if eta.EstimatedMinutes <= 0 {
		t.Errorf("Expected positive estimated minutes, got %d", eta.EstimatedMinutes)
	}

	if eta.Confidence <= 0 || eta.Confidence > 1 {
		t.Errorf("Expected confidence between 0 and 1, got %f", eta.Confidence)
	}

	if eta.ETADate.Before(now) {
		t.Errorf("ETA date should be in the future, got %v", eta.ETADate)
	}

	if eta.ETADateHigh.Before(eta.ETADate) {
		t.Errorf("High estimate should be >= ETA date")
	}

	if len(eta.Factors) == 0 {
		t.Error("Expected at least one factor")
	}
}

func TestEstimateETAForIssue_NotFound(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{}

	_, err := EstimateETAForIssue(issues, nil, "nonexistent", 1, now)
	if err == nil {
		t.Error("Expected error for nonexistent issue")
	}
}

func TestEstimateETAForIssue_WithExplicitEstimate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	explicitMinutes := 120

	issues := []model.Issue{
		{
			ID:               "test-1",
			Title:            "Test issue with estimate",
			Status:           model.StatusOpen,
			IssueType:        model.TypeTask,
			EstimatedMinutes: &explicitMinutes,
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	// Explicit estimate should be higher confidence
	if eta.Confidence < 0.3 {
		t.Errorf("Expected higher confidence with explicit estimate, got %f", eta.Confidence)
	}

	// Should mention explicit estimate in factors
	hasExplicitFactor := false
	for _, f := range eta.Factors {
		if len(f) > 0 && f[:8] == "estimate" {
			hasExplicitFactor = true
			break
		}
	}
	if !hasExplicitFactor {
		t.Error("Expected factor mentioning estimate")
	}
}

func TestEstimateETAForIssue_TypeWeights(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	taskIssue := model.Issue{
		ID:        "task-1",
		Title:     "Task",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
	}

	epicIssue := model.Issue{
		ID:        "epic-1",
		Title:     "Epic",
		Status:    model.StatusOpen,
		IssueType: model.TypeEpic,
	}

	taskETA, _ := EstimateETAForIssue([]model.Issue{taskIssue}, nil, "task-1", 1, now)
	epicETA, _ := EstimateETAForIssue([]model.Issue{epicIssue}, nil, "epic-1", 1, now)

	// Epic should take longer than task (type weight is higher)
	if epicETA.EstimatedDays <= taskETA.EstimatedDays {
		t.Errorf("Epic should have longer ETA than task: epic=%f, task=%f",
			epicETA.EstimatedDays, taskETA.EstimatedDays)
	}
}

func TestEstimateETAForIssue_MultipleAgents(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "Test issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
		},
	}

	eta1, _ := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	eta2, _ := EstimateETAForIssue(issues, nil, "test-1", 2, now)

	// 2 agents should complete faster (roughly half the time)
	if eta2.EstimatedDays >= eta1.EstimatedDays {
		t.Errorf("2 agents should be faster than 1: 1 agent=%f days, 2 agents=%f days",
			eta1.EstimatedDays, eta2.EstimatedDays)
	}
}

func TestEstimateETAForIssue_VelocityFromClosures(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	closedAt := now.Add(-7 * 24 * time.Hour) // 7 days ago

	issues := []model.Issue{
		{
			ID:        "open-1",
			Title:     "Open issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
		},
		{
			ID:        "closed-1",
			Title:     "Closed issue",
			Status:    model.StatusClosed,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
			ClosedAt:  &closedAt,
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "open-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	// Should have velocity factor from closure history
	hasVelocityFactor := false
	for _, f := range eta.Factors {
		if len(f) >= 8 && f[:8] == "velocity" {
			hasVelocityFactor = true
			break
		}
	}
	if !hasVelocityFactor {
		t.Error("Expected velocity factor from closure history")
	}
}

func TestComputeMedianEstimatedMinutes(t *testing.T) {
	// No estimates - should return default
	emptyIssues := []model.Issue{{ID: "1"}}
	median := computeMedianEstimatedMinutes(emptyIssues)
	if median != DefaultEstimatedMinutes {
		t.Errorf("Expected default %d for empty estimates, got %d", DefaultEstimatedMinutes, median)
	}

	// Odd number of estimates
	est30, est60, est90 := 30, 60, 90
	oddIssues := []model.Issue{
		{ID: "1", EstimatedMinutes: &est30},
		{ID: "2", EstimatedMinutes: &est60},
		{ID: "3", EstimatedMinutes: &est90},
	}
	median = computeMedianEstimatedMinutes(oddIssues)
	if median != 60 {
		t.Errorf("Expected median 60 for odd count, got %d", median)
	}

	// Even number of estimates
	est120 := 120
	evenIssues := []model.Issue{
		{ID: "1", EstimatedMinutes: &est30},
		{ID: "2", EstimatedMinutes: &est60},
		{ID: "3", EstimatedMinutes: &est90},
		{ID: "4", EstimatedMinutes: &est120},
	}
	median = computeMedianEstimatedMinutes(evenIssues)
	// Median of [30, 60, 90, 120] = (60 + 90) / 2 = 75
	if median != 75 {
		t.Errorf("Expected median 75 for even count, got %d", median)
	}
}

func TestClampFloat(t *testing.T) {
	if clampFloat(0.5, 0.0, 1.0) != 0.5 {
		t.Error("Value in range should not change")
	}
	if clampFloat(-0.5, 0.0, 1.0) != 0.0 {
		t.Error("Value below range should be clamped to lo")
	}
	if clampFloat(1.5, 0.0, 1.0) != 1.0 {
		t.Error("Value above range should be clamped to hi")
	}
}

func TestDurationDays(t *testing.T) {
	if durationDays(0) != 0 {
		t.Error("0 days should return 0 duration")
	}
	if durationDays(-1) != 0 {
		t.Error("negative days should return 0 duration")
	}

	oneDay := durationDays(1)
	expected := 24 * time.Hour
	if oneDay != expected {
		t.Errorf("Expected %v for 1 day, got %v", expected, oneDay)
	}
}

// TestHasLabel is in label_health_test.go
