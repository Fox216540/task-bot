package bot_test

import (
	"reflect"
	"testing"
	"time"

	"task-bot/internal/bot"
)

func TestParseFreeCommand(t *testing.T) {
	days, start, end, err := bot.ParseFreeCommand("/free mon,wed 16:00-18:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDays := []int{1, 3}
	if !reflect.DeepEqual(days, wantDays) {
		t.Fatalf("days mismatch: got %v want %v", days, wantDays)
	}
	if start != "16:00" || end != "18:00" {
		t.Fatalf("range mismatch: got %s-%s", start, end)
	}
}

func TestParseBusyCommandNow(t *testing.T) {
	now := time.Date(2026, 4, 8, 14, 35, 0, 0, time.UTC)
	days, start, end, isNow, err := bot.ParseBusyCommand("/busy now", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNow {
		t.Fatalf("expected isNow=true")
	}
	if !reflect.DeepEqual(days, []int{3}) {
		t.Fatalf("days mismatch: %v", days)
	}
	if start != "14:35" || end != "23:59" {
		t.Fatalf("range mismatch: %s-%s", start, end)
	}
}

func TestParseTaskCommand(t *testing.T) {
	name, duration, taskType, priority, err := bot.ParseTaskCommand("/task Сделать API | 30 | с | 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Сделать API" || duration != 30 || taskType != "deep" || priority != 1 {
		t.Fatalf("unexpected parsed values: %q %d %q %d", name, duration, taskType, priority)
	}
}

func TestParseTaskCommandInvalid(t *testing.T) {
	_, _, _, _, err := bot.ParseTaskCommand("/task foo | -1 | c | 1")
	if err == nil {
		t.Fatalf("expected error for negative duration")
	}
}

func TestParseBusyCommandRejectsMultipleDays(t *testing.T) {
	now := time.Date(2026, 4, 8, 14, 35, 0, 0, time.UTC)
	_, _, _, _, err := bot.ParseBusyCommand("/busy mon,wed 10:00-11:00", now)
	if err == nil {
		t.Fatalf("expected error for multiple busy days")
	}
}
