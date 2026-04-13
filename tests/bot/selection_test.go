package bot_test

import (
	"testing"
	"time"

	"task-bot/internal/bot"
	"task-bot/internal/models"
)

func TestSelectTaskPrefersForcedTaskThatFits(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	tasks := []models.Task{
		{ID: 1, Name: "regular", Duration: 20, Type: "medium", Priority: 1, IgnoreCount: 1},
		{ID: 2, Name: "forced", Duration: 15, Type: "light", Priority: 4, IgnoreCount: 8},
	}

	selected := bot.SelectTask(tasks, 30, now)
	if selected == nil {
		t.Fatalf("expected selected task")
	}
	if selected.ID != 2 {
		t.Fatalf("expected forced task id=2, got id=%d", selected.ID)
	}
}

func TestSelectTaskSkipsFutureScheduled(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	tasks := []models.Task{
		{ID: 1, Name: "future", Duration: 10, Type: "quick", Priority: 1, IgnoreCount: 20, ScheduledAt: &future},
		{ID: 2, Name: "now", Duration: 10, Type: "quick", Priority: 2, IgnoreCount: 1},
	}

	selected := bot.SelectTask(tasks, 20, now)
	if selected == nil {
		t.Fatalf("expected selected task")
	}
	if selected.ID != 2 {
		t.Fatalf("expected non-future task id=2, got id=%d", selected.ID)
	}
}

func TestSelectTaskFallsBackToShortestFit(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	tasks := []models.Task{
		{ID: 1, Name: "a", Duration: 20, Type: "deep", Priority: 1, IgnoreCount: 0},
		{ID: 2, Name: "b", Duration: 8, Type: "quick", Priority: 5, IgnoreCount: 0},
	}

	selected := bot.SelectTask(tasks, 12, now)
	if selected == nil {
		t.Fatalf("expected selected task")
	}
	if selected.ID != 2 {
		t.Fatalf("expected shortest fitting task id=2, got id=%d", selected.ID)
	}
}

func TestSelectTaskIgnoresBufferForFitCheck(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	tasks := []models.Task{
		{ID: 21, Name: "deep-exact", Duration: 30, Type: "deep", Priority: 1, IgnoreCount: 0},
	}

	selected := bot.SelectTask(tasks, 30, now)
	if selected == nil {
		t.Fatalf("expected selected task when duration fits exactly")
	}
	if selected.ID != 21 {
		t.Fatalf("expected task id=21, got id=%d", selected.ID)
	}
}

func TestSelectTaskReturnsNilWhenNoTaskFits(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	tasks := []models.Task{
		{ID: 10, Name: "long-a", Duration: 40, Type: "deep", Priority: 1, IgnoreCount: 0},
		{ID: 11, Name: "long-b", Duration: 25, Type: "medium", Priority: 2, IgnoreCount: 0},
		{ID: 12, Name: "shortest", Duration: 15, Type: "deep", Priority: 3, IgnoreCount: 0},
	}

	selected := bot.SelectTask(tasks, 10, now)
	if selected != nil {
		t.Fatalf("expected nil when no task fits, got id=%d", selected.ID)
	}
}

func TestShouldRemindEscalationIntervals(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 10, 0, 0, time.UTC)

	last := now.Add(-2 * time.Minute)
	t1 := &models.Task{IgnoreCount: 0, LastRemindedAt: &last}
	if bot.ShouldRemind(t1, now) {
		t.Fatalf("ignore 0 should require 3 minutes")
	}

	t2 := &models.Task{IgnoreCount: 5, LastRemindedAt: &last}
	if !bot.ShouldRemind(t2, now) {
		t.Fatalf("ignore 5 should remind after 2 minutes")
	}

	lastShort := now.Add(-50 * time.Second)
	t3 := &models.Task{IgnoreCount: 10, LastRemindedAt: &lastShort}
	if bot.ShouldRemind(t3, now) {
		t.Fatalf("ignore 10 should require 1 minute")
	}
}
