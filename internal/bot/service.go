package bot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"task-bot/internal/models"
	"task-bot/internal/storage"
)

const defaultForcedThreshold = 8

func SelectTask(tasks []models.Task, slotMinutes int, now time.Time) *models.Task {
	return selectTask(tasks, slotMinutes, now, defaultForcedThreshold)
}

func ShouldRemind(t *models.Task, now time.Time) bool {
	return shouldRemind(t, now)
}

type Service struct {
	repo            *storage.SQLiteRepo
	api             *APIClient
	allowedUserID   int64
	forcedThreshold int
}

func NewService(repo *storage.SQLiteRepo, api *APIClient, allowedUserID int64, forcedThreshold int) *Service {
	return &Service{
		repo:            repo,
		api:             api,
		allowedUserID:   allowedUserID,
		forcedThreshold: forcedThreshold,
	}
}

func (s *Service) RunPolling(ctx context.Context) error {
	var offset int
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, err := s.api.GetUpdates(ctx, offset)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			if err := s.handleUpdate(ctx, upd); err != nil {
				log.Printf("handle update error: %v", err)
			}
		}
	}
}

func (s *Service) RunReminderCycle(ctx context.Context) error {
	userIDs, err := s.repo.GetUserIDs(ctx)
	if err != nil {
		return err
	}
	now := time.Now()

	for _, chatID := range userIDs {
		if !s.isAllowedChatID(chatID) {
			continue
		}
		if err := s.processUserReminder(ctx, chatID, now); err != nil {
			log.Printf("process user %d reminder error: %v", chatID, err)
		}
	}
	return nil
}

func (s *Service) handleUpdate(ctx context.Context, upd Update) error {
	if !s.isAllowedUpdate(upd) {
		chatID, hasChat := updateChatID(upd)
		if hasChat {
			_ = s.api.SendMessage(ctx, chatID, "Access denied", nil)
		}
		return nil
	}

	if upd.Message != nil && strings.TrimSpace(upd.Message.Text) != "" {
		return s.handleMessage(ctx, upd.Message)
	}
	if upd.CallbackQuery != nil {
		return s.handleCallback(ctx, upd.CallbackQuery)
	}
	return nil
}

func (s *Service) handleMessage(ctx context.Context, m *Message) error {
	chatID := m.Chat.ID
	if err := s.repo.EnsureUser(ctx, chatID); err != nil {
		return err
	}
	lang, err := s.repo.GetLang(ctx, chatID)
	if err != nil {
		return err
	}

	text := strings.TrimSpace(m.Text)
	switch {
	case text == "/start":
		return s.api.SendMessage(ctx, chatID, tr(lang, "welcome"), nil)
	case strings.HasPrefix(text, "/lang"):
		parsedLang, err := parseLang(text)
		if err != nil {
			return s.api.SendMessage(ctx, chatID, tr(lang, "usage_lang"), nil)
		}
		if err := s.repo.SetLang(ctx, chatID, parsedLang); err != nil {
			return err
		}
		return s.api.SendMessage(ctx, chatID, tr(parsedLang, "lang_set"), nil)
	case text == "/info":
		state, _ := s.repo.DebugState(ctx, chatID)
		return s.api.SendMessage(ctx, chatID, tr(lang, "info")+"\n\nState: "+state, nil)
	case strings.HasPrefix(text, "/free"):
		return s.handleFree(ctx, chatID, lang, text)
	case strings.HasPrefix(text, "/busy"):
		return s.handleBusy(ctx, chatID, lang, text)
	case text == "/unbusy":
		return s.handleUnbusy(ctx, chatID, lang)
	case strings.HasPrefix(text, "/task"):
		return s.handleTask(ctx, chatID, lang, text)
	default:
		return s.api.SendMessage(ctx, chatID, tr(lang, "bad_command"), nil)
	}
}

func (s *Service) isAllowedUpdate(upd Update) bool {
	userID, ok := updateUserID(upd)
	if !ok {
		return false
	}
	if userID != s.allowedUserID {
		return false
	}

	chatID, hasChat := updateChatID(upd)
	if hasChat && chatID != s.allowedUserID {
		return false
	}

	return true
}

func (s *Service) isAllowedChatID(chatID int64) bool {
	return chatID == s.allowedUserID
}

func updateUserID(upd Update) (int64, bool) {
	if upd.Message != nil && upd.Message.From != nil {
		return upd.Message.From.ID, true
	}
	if upd.CallbackQuery != nil && upd.CallbackQuery.From != nil {
		return upd.CallbackQuery.From.ID, true
	}
	return 0, false
}

func updateChatID(upd Update) (int64, bool) {
	if upd.Message != nil {
		return upd.Message.Chat.ID, true
	}
	if upd.CallbackQuery != nil && upd.CallbackQuery.Message != nil {
		return upd.CallbackQuery.Message.Chat.ID, true
	}
	return 0, false
}

func (s *Service) handleFree(ctx context.Context, chatID int64, lang, text string) error {
	days, start, end, err := parseFreeCommand(text)
	if err != nil {
		return s.api.SendMessage(ctx, chatID, tr(lang, "usage_free"), nil)
	}
	if err := s.repo.UpsertFreeSlots(ctx, chatID, days, start, end); err != nil {
		return err
	}
	return s.api.SendMessage(ctx, chatID, tr(lang, "free_saved"), nil)
}

func (s *Service) handleBusy(ctx context.Context, chatID int64, lang, text string) error {
	days, start, end, isNow, err := parseBusyCommand(text, time.Now())
	if err != nil {
		return s.api.SendMessage(ctx, chatID, tr(lang, "usage_busy"), nil)
	}
	for _, day := range days {
		if err := s.repo.AddBusySlot(ctx, chatID, day, start, end, isNow); err != nil {
			return err
		}
	}
	key := "busy_saved"
	if isNow {
		key = "busy_now_saved"
	}
	return s.api.SendMessage(ctx, chatID, tr(lang, key), nil)
}

func (s *Service) handleUnbusy(ctx context.Context, chatID int64, lang string) error {
	if err := s.repo.CloseOpenBusy(ctx, chatID, time.Now().Format("15:04")); err != nil {
		return err
	}
	if err := s.api.SendMessage(ctx, chatID, tr(lang, "unbusy_done"), nil); err != nil {
		return err
	}
	return s.processUserReminder(ctx, chatID, time.Now())
}

func (s *Service) handleTask(ctx context.Context, chatID int64, lang, text string) error {
	name, duration, typ, priority, err := parseTaskCommand(text)
	if err != nil {
		return s.api.SendMessage(ctx, chatID, tr(lang, "usage_task"), nil)
	}

	task := models.Task{
		ChatID:      chatID,
		Name:        name,
		Duration:    duration,
		Type:        typ,
		Priority:    priority,
		IgnoreCount: 0,
	}
	if err := s.repo.AddTask(ctx, task); err != nil {
		return err
	}
	if err := s.api.SendMessage(ctx, chatID, tr(lang, "task_saved"), nil); err != nil {
		return err
	}
	return s.processUserReminder(ctx, chatID, time.Now())
}

func (s *Service) handleCallback(ctx context.Context, cb *CallbackQuery) error {
	if cb.Message == nil {
		return nil
	}
	chatID := cb.Message.Chat.ID
	if err := s.repo.EnsureUser(ctx, chatID); err != nil {
		return err
	}
	lang, _ := s.repo.GetLang(ctx, chatID)

	parts := strings.Split(cb.Data, ":")
	if len(parts) != 2 {
		return s.api.AnswerCallback(ctx, cb.ID, "invalid action")
	}
	taskID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return s.api.AnswerCallback(ctx, cb.ID, "invalid task id")
	}

	task, err := s.repo.GetTaskByID(ctx, chatID, taskID)
	if err != nil {
		return err
	}
	if task == nil || task.Done {
		return s.api.AnswerCallback(ctx, cb.ID, tr(lang, "no_task"))
	}

	switch parts[0] {
	case "start":
		if err := s.repo.StartTask(ctx, chatID, taskID); err != nil {
			return err
		}
		if err := s.api.AnswerCallback(ctx, cb.ID, tr(lang, "start_ok")); err != nil {
			return err
		}
		return s.api.SendMessage(ctx, chatID, tr(lang, "start_ok"), nil)
	case "done":
		if !task.Started {
			return s.api.AnswerCallback(ctx, cb.ID, tr(lang, "start_required"))
		}
		if err := s.repo.CompleteTask(ctx, chatID, taskID); err != nil {
			return err
		}
		if err := s.api.AnswerCallback(ctx, cb.ID, tr(lang, "done_ok")); err != nil {
			return err
		}
		if err := s.api.SendMessage(ctx, chatID, tr(lang, "done_ok"), nil); err != nil {
			return err
		}
		return s.processUserReminder(ctx, chatID, time.Now())
	default:
		return s.api.AnswerCallback(ctx, cb.ID, "unknown action")
	}
}

func (s *Service) processUserReminder(ctx context.Context, chatID int64, now time.Time) error {
	lang, _ := s.repo.GetLang(ctx, chatID)

	busy, err := s.repo.IsBusyNow(ctx, chatID, now)
	if err != nil {
		return err
	}
	tasks, err := s.repo.GetCandidateTasks(ctx, chatID)
	if err != nil {
		return err
	}
	if busy {
		return s.rescheduleOverdue(ctx, chatID, tasks, now)
	}

	remaining, err := s.repo.CurrentSlotRemainingMinutes(ctx, chatID, now)
	if err != nil {
		return err
	}
	if remaining <= 0 {
		return nil
	}

	active, err := s.repo.GetCurrentActiveTask(ctx, chatID)
	if err != nil {
		return err
	}
	if active != nil {
		activeTotal := active.Duration
		if activeTotal <= remaining {
			return nil
		}
		// Dynamic switching: active task does not fit anymore, release it.
		if err := s.repo.StopActiveTask(ctx, chatID); err != nil {
			return err
		}
		for i := range tasks {
			if tasks[i].ID == active.ID {
				tasks[i].Started = false
				break
			}
		}
	}

	selected := selectTask(tasks, remaining, now, s.forcedThreshold)
	if selected == nil {
		return nil
	}
	if selected.ScheduledAt != nil && selected.ScheduledAt.After(now) {
		return nil
	}

	if !shouldRemind(selected, now) {
		return nil
	}

	ignore := selected.IgnoreCount + 1
	priority := selected.Priority
	if ignore%3 == 0 && priority > 1 {
		priority--
	}
	if err := s.repo.UpdateTaskReminder(ctx, chatID, selected.ID, now, ignore, priority); err != nil {
		return err
	}

	text := fmt.Sprintf(tr(lang, "hard_prompt"), selected.Name, selected.Duration)
	keyboard := [][]Button{
		{
			{Text: "🚀", CallbackData: fmt.Sprintf("start:%d", selected.ID)},
			{Text: "✅", CallbackData: fmt.Sprintf("done:%d", selected.ID)},
		},
	}
	return s.api.SendMessage(ctx, chatID, text, keyboard)
}

func (s *Service) rescheduleOverdue(ctx context.Context, chatID int64, tasks []models.Task, now time.Time) error {
	next, ok, err := s.repo.NextFreeSlotStart(ctx, chatID, now)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	for _, t := range tasks {
		if t.ScheduledAt != nil && (t.ScheduledAt.Before(now) || t.ScheduledAt.Equal(now)) {
			if err := s.repo.UpdateTaskSchedule(ctx, chatID, t.ID, next); err != nil {
				return err
			}
		}
	}
	return nil
}

func selectTask(tasks []models.Task, slotMinutes int, now time.Time, forcedThreshold int) *models.Task {
	if len(tasks) == 0 {
		return nil
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		a := tasks[i]
		b := tasks[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.IgnoreCount != b.IgnoreCount {
			return a.IgnoreCount > b.IgnoreCount
		}
		return scheduledTime(a, now).Before(scheduledTime(b, now))
	})

	var forcedFit []models.Task
	var regularFit []models.Task
	var shortestFit *models.Task
	for i := range tasks {
		t := tasks[i]
		if t.Done || t.Started {
			continue
		}
		if t.ScheduledAt != nil && t.ScheduledAt.After(now) {
			continue
		}
		total := t.Duration
		if total > slotMinutes {
			continue
		}
		if shortestFit == nil || t.Duration < shortestFit.Duration {
			tmp := t
			shortestFit = &tmp
		}
		if t.IgnoreCount >= forcedThreshold {
			forcedFit = append(forcedFit, t)
		} else {
			regularFit = append(regularFit, t)
		}
	}

	if len(forcedFit) > 0 {
		return &forcedFit[0]
	}
	if len(regularFit) > 0 {
		return &regularFit[0]
	}
	return shortestFit
}

func scheduledTime(t models.Task, now time.Time) time.Time {
	if t.ScheduledAt == nil {
		return now
	}
	return *t.ScheduledAt
}

func shouldRemind(t *models.Task, now time.Time) bool {
	if t.LastRemindedAt == nil {
		return true
	}
	interval := 3 * time.Minute
	switch {
	case t.IgnoreCount >= 10:
		interval = 1 * time.Minute
	case t.IgnoreCount >= 5:
		interval = 2 * time.Minute
	}
	return now.Sub(*t.LastRemindedAt) >= interval
}
