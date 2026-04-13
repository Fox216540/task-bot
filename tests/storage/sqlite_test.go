package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"
)

func newTestRepo(t *testing.T) *storage.SQLiteRepo {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	repo, err := storage.NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})
	return repo
}

func TestUserLangLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)

	const chatID int64 = 42
	if err := repo.EnsureUser(ctx, chatID); err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	lang, err := repo.GetLang(ctx, chatID)
	if err != nil {
		t.Fatalf("get lang: %v", err)
	}
	if lang != "ru" {
		t.Fatalf("default lang mismatch: got %q", lang)
	}

	if err := repo.SetLang(ctx, chatID, "en"); err != nil {
		t.Fatalf("set lang: %v", err)
	}
	lang, err = repo.GetLang(ctx, chatID)
	if err != nil {
		t.Fatalf("get lang after set: %v", err)
	}
	if lang != "en" {
		t.Fatalf("lang mismatch after set: got %q", lang)
	}
}

func TestSlotsAndBusyLogic(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 100

	if err := repo.UpsertFreeSlots(ctx, chatID, []int{3}, "10:00", "12:00"); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	now := time.Date(2026, 4, 8, 10, 30, 0, 0, time.UTC)
	remaining, err := repo.CurrentSlotRemainingMinutes(ctx, chatID, now)
	if err != nil {
		t.Fatalf("current slot remaining: %v", err)
	}
	if remaining <= 80 || remaining > 90 {
		t.Fatalf("unexpected remaining minutes: %d", remaining)
	}

	if err := repo.AddBusySlot(ctx, chatID, 3, "10:00", "11:00", false); err != nil {
		t.Fatalf("add busy slot: %v", err)
	}

	isBusy, err := repo.IsBusyNow(ctx, chatID, now)
	if err != nil {
		t.Fatalf("is busy now: %v", err)
	}
	if !isBusy {
		t.Fatalf("expected busy=true")
	}

	if err := repo.AddBusySlot(ctx, chatID, 3, "10:20", "23:59", true); err != nil {
		t.Fatalf("add open busy: %v", err)
	}
	if err := repo.CloseOpenBusy(ctx, chatID, "10:40"); err != nil {
		t.Fatalf("close open busy: %v", err)
	}
}

func TestAddBusySlotMergesOverlaps(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 111

	if err := repo.AddBusySlot(ctx, chatID, 3, "10:00", "11:00", false); err != nil {
		t.Fatalf("add busy slot #1: %v", err)
	}
	if err := repo.AddBusySlot(ctx, chatID, 3, "10:30", "12:00", false); err != nil {
		t.Fatalf("add busy slot #2: %v", err)
	}

	// 11:30 should still be busy due to merged interval.
	at1130 := time.Date(2026, 4, 8, 11, 30, 0, 0, time.UTC)
	isBusy, err := repo.IsBusyNow(ctx, chatID, at1130)
	if err != nil {
		t.Fatalf("is busy at 11:30: %v", err)
	}
	if !isBusy {
		t.Fatalf("expected merged busy interval to include 11:30")
	}
}

func TestTaskLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 7

	if err := repo.AddTask(ctx, models.Task{ChatID: chatID, Name: "Task A", Duration: 25, Type: "deep", Priority: 2, IgnoreCount: 0}); err != nil {
		t.Fatalf("add task A: %v", err)
	}
	if err := repo.AddTask(ctx, models.Task{ChatID: chatID, Name: "Task B", Duration: 10, Type: "quick", Priority: 1, IgnoreCount: 3}); err != nil {
		t.Fatalf("add task B: %v", err)
	}

	tasks, err := repo.GetCandidateTasks(ctx, chatID)
	if err != nil {
		t.Fatalf("get candidate tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks count mismatch: got %d", len(tasks))
	}

	var taskBID int64
	for _, task := range tasks {
		if task.Name == "Task B" {
			taskBID = task.ID
		}
	}
	if taskBID == 0 {
		t.Fatalf("task B id not found")
	}

	if err := repo.StartTask(ctx, chatID, taskBID); err != nil {
		t.Fatalf("start task: %v", err)
	}
	active, err := repo.GetCurrentActiveTask(ctx, chatID)
	if err != nil {
		t.Fatalf("get active task: %v", err)
	}
	if active == nil || active.ID != taskBID {
		t.Fatalf("unexpected active task: %+v", active)
	}

	now := time.Date(2026, 4, 8, 11, 0, 0, 0, time.UTC)
	if err := repo.UpdateTaskReminder(ctx, chatID, taskBID, now, 4, 1); err != nil {
		t.Fatalf("update reminder: %v", err)
	}
	taskB, err := repo.GetTaskByID(ctx, chatID, taskBID)
	if err != nil {
		t.Fatalf("get task by id: %v", err)
	}
	if taskB == nil || taskB.IgnoreCount != 4 || taskB.Priority != 1 {
		t.Fatalf("task reminder state mismatch: %+v", taskB)
	}

	if err := repo.CompleteTask(ctx, chatID, taskBID); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	active, err = repo.GetCurrentActiveTask(ctx, chatID)
	if err != nil {
		t.Fatalf("get active after complete: %v", err)
	}
	if active != nil {
		t.Fatalf("expected no active task after complete")
	}
}

func TestStartTaskReturnsErrorForUnknownID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 222

	if err := repo.AddTask(ctx, models.Task{ChatID: chatID, Name: "T", Duration: 10, Type: "quick", Priority: 1}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	if err := repo.StartTask(ctx, chatID, 999999); err == nil {
		t.Fatalf("expected error for unknown task id")
	}
}

func TestCompleteTaskRequiresStartedState(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 333

	if err := repo.AddTask(ctx, models.Task{ChatID: chatID, Name: "T", Duration: 10, Type: "quick", Priority: 1}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	tasks, err := repo.GetCandidateTasks(ctx, chatID)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("get task id failed: %v", err)
	}

	if err := repo.CompleteTask(ctx, chatID, tasks[0].ID); err == nil {
		t.Fatalf("expected error when completing non-started task")
	}
}

func TestNextFreeSlotStart(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 9

	if err := repo.UpsertFreeSlots(ctx, chatID, []int{1, 5}, "09:00", "10:00"); err != nil {
		t.Fatalf("upsert free slots: %v", err)
	}

	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	next, ok, err := repo.NextFreeSlotStart(ctx, chatID, now)
	if err != nil {
		t.Fatalf("next free slot start: %v", err)
	}
	if !ok {
		t.Fatalf("expected to find next slot")
	}
	if next.Weekday() != time.Friday {
		t.Fatalf("unexpected next weekday: %s", next.Weekday())
	}
}

func TestCurrentSlotRemainingIsClampedByUpcomingBusy(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 444

	if err := repo.UpsertFreeSlots(ctx, chatID, []int{3}, "10:00", "12:00"); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := repo.AddBusySlot(ctx, chatID, 3, "10:45", "11:15", false); err != nil {
		t.Fatalf("add busy: %v", err)
	}

	now := time.Date(2026, 4, 8, 10, 30, 0, 0, time.UTC)
	remaining, err := repo.CurrentSlotRemainingMinutes(ctx, chatID, now)
	if err != nil {
		t.Fatalf("remaining minutes: %v", err)
	}
	if remaining > 15 || remaining < 14 {
		t.Fatalf("expected remaining near 15 minutes due to upcoming busy, got %d", remaining)
	}
}

func TestOpenBusySurvivesDayChangeUntilUnbusy(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	const chatID int64 = 77

	if err := repo.AddBusySlot(ctx, chatID, 3, "23:00", "23:59", true); err != nil {
		t.Fatalf("add open busy: %v", err)
	}

	nextDay := time.Date(2026, 4, 9, 1, 0, 0, 0, time.UTC) // Thursday
	isBusy, err := repo.IsBusyNow(ctx, chatID, nextDay)
	if err != nil {
		t.Fatalf("is busy after day change: %v", err)
	}
	if !isBusy {
		t.Fatalf("expected busy=true for open busy after midnight")
	}

	if err := repo.CloseOpenBusy(ctx, chatID, "01:05"); err != nil {
		t.Fatalf("close open busy: %v", err)
	}
	isBusy, err = repo.IsBusyNow(ctx, chatID, nextDay)
	if err != nil {
		t.Fatalf("is busy after unbusy: %v", err)
	}
	if isBusy {
		t.Fatalf("expected busy=false after unbusy")
	}
}

func TestSchemaUsesExpectedTableNames(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema.sqlite")
	repo, err := storage.NewSQLiteRepo(dbPath)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	_ = repo.Close()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm for schema check: %v", err)
	}

	for _, table := range []string{"users", "free_slots", "busy_slots", "tasks"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}
}
