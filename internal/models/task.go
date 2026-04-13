package models

import "time"

type Task struct {
	ID             int64
	ChatID         int64
	Name           string
	Duration       int
	Type           string
	Priority       int
	ScheduledAt    *time.Time
	Started        bool
	Done           bool
	IgnoreCount    int
	LastRemindedAt *time.Time
}

func BufferByType(taskType string) int {
	switch taskType {
	case "c", "с", "deep":
		return 15
	case "m", "м", "medium":
		return 10
	case "l", "л", "light":
		return 5
	case "b", "б", "quick":
		return 2
	default:
		return 5
	}
}
