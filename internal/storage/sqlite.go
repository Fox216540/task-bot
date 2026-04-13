package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"


	"task-bot/internal/models"
)

type UserEntity struct {
	ChatID int64  `gorm:"primaryKey;column:chat_id"`
	Lang   string `gorm:"default:ru"`
}

type FreeSlotEntity struct {
	ID     uint   `gorm:"primaryKey"`
	ChatID int64  `gorm:"column:chat_id;index;uniqueIndex:idx_free_unique"`
	Day    int    `gorm:"uniqueIndex:idx_free_unique"`
	Start  string `gorm:"uniqueIndex:idx_free_unique"`
	End    string `gorm:"uniqueIndex:idx_free_unique"`
}

type BusySlotEntity struct {
	ID     uint  `gorm:"primaryKey"`
	ChatID int64 `gorm:"column:chat_id;index"`
	Day    int
	Start  string
	End    string
	IsOpen bool `gorm:"column:is_open"`
}

type TaskEntity struct {
	ID             uint  `gorm:"primaryKey"`
	ChatID         int64 `gorm:"column:chat_id;index"`
	Name           string
	Duration       int
	Type           string
	Priority       int
	ScheduledAt    *time.Time `gorm:"column:scheduled_at;index"`
	Started        bool
	Done           bool       `gorm:"index"`
	IgnoreCount    int        `gorm:"column:ignore_count"`
	LastRemindedAt *time.Time `gorm:"column:last_reminded_at"`
}

func (UserEntity) TableName() string {
	return "users"
}

func (FreeSlotEntity) TableName() string {
	return "free_slots"
}

func (BusySlotEntity) TableName() string {
	return "busy_slots"
}

func (TaskEntity) TableName() string {
	return "tasks"
}

type SQLiteRepo struct {
	db *gorm.DB
}

func NewSQLiteRepo(path string) (*SQLiteRepo, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&UserEntity{}, &FreeSlotEntity{}, &BusySlotEntity{}, &TaskEntity{}); err != nil {
		return nil, err
	}

	return &SQLiteRepo{db: db}, nil
}

func (r *SQLiteRepo) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (r *SQLiteRepo) EnsureUser(ctx context.Context, chatID int64) error {
	entity := UserEntity{ChatID: chatID}
	return r.db.WithContext(ctx).Attrs(UserEntity{Lang: "ru"}).FirstOrCreate(&entity).Error
}

func (r *SQLiteRepo) SetLang(ctx context.Context, chatID int64, lang string) error {
	return r.db.WithContext(ctx).
		Model(&UserEntity{}).
		Where(&UserEntity{ChatID: chatID}).
		Update("lang", lang).Error
}

func (r *SQLiteRepo) GetLang(ctx context.Context, chatID int64) (string, error) {
	var entity UserEntity
	err := r.db.WithContext(ctx).First(&entity, "chat_id = ?", chatID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "ru", nil
	}
	if err != nil {
		return "", err
	}
	if entity.Lang == "" {
		return "ru", nil
	}
	return entity.Lang, nil
}

func (r *SQLiteRepo) UpsertFreeSlots(ctx context.Context, chatID int64, days []int, start, end string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, day := range days {
			entity := FreeSlotEntity{ChatID: chatID, Day: day, Start: start, End: end}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "chat_id"},
					{Name: "day"},
					{Name: "start"},
					{Name: "end"},
				},
				DoNothing: true,
			}).Create(&entity).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SQLiteRepo) AddBusySlot(ctx context.Context, chatID int64, day int, start, end string, isOpen bool) error {
	if isOpen {
		var open BusySlotEntity
		err := r.db.WithContext(ctx).
			Where("chat_id = ? AND is_open = ?", chatID, true).
			First(&open).Error
		if err == nil {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	if !isOpen {
		var overlaps []BusySlotEntity
		if err := r.db.WithContext(ctx).
			Where("chat_id = ? AND day = ? AND is_open = ?", chatID, day, false).
			Where("start <= ? AND end >= ?", end, start).
			Find(&overlaps).Error; err != nil {
			return err
		}
		if len(overlaps) > 0 {
			mergedStart := start
			mergedEnd := end
			ids := make([]uint, 0, len(overlaps))
			for _, slot := range overlaps {
				ids = append(ids, slot.ID)
				if slot.Start < mergedStart {
					mergedStart = slot.Start
				}
				if slot.End > mergedEnd {
					mergedEnd = slot.End
				}
			}
			if err := r.db.WithContext(ctx).
				Model(&BusySlotEntity{}).
				Where("id = ?", ids[0]).
				Updates(map[string]any{"start": mergedStart, "end": mergedEnd}).Error; err != nil {
				return err
			}
			if len(ids) > 1 {
				if err := r.db.WithContext(ctx).
					Where("id IN ?", ids[1:]).
					Delete(&BusySlotEntity{}).Error; err != nil {
					return err
				}
			}
			return nil
		}
	}

	entity := BusySlotEntity{ChatID: chatID, Day: day, Start: start, End: end, IsOpen: isOpen}
	return r.db.WithContext(ctx).Create(&entity).Error
}

func (r *SQLiteRepo) CloseOpenBusy(ctx context.Context, chatID int64, end string) error {
	return r.db.WithContext(ctx).
		Model(&BusySlotEntity{}).
		Where(&BusySlotEntity{ChatID: chatID, IsOpen: true}).
		Updates(map[string]any{"is_open": false, "end": end}).Error
}

func (r *SQLiteRepo) AddTask(ctx context.Context, t models.Task) error {
	scheduledAt := t.ScheduledAt
	if scheduledAt == nil {
		now := time.Now().UTC()
		scheduledAt = &now
	}

	entity := TaskEntity{
		ChatID:         t.ChatID,
		Name:           t.Name,
		Duration:       t.Duration,
		Type:           t.Type,
		Priority:       t.Priority,
		ScheduledAt:    scheduledAt,
		Started:        t.Started,
		Done:           t.Done,
		IgnoreCount:    t.IgnoreCount,
		LastRemindedAt: t.LastRemindedAt,
	}
	return r.db.WithContext(ctx).Create(&entity).Error
}

func (r *SQLiteRepo) GetCurrentActiveTask(ctx context.Context, chatID int64) (*models.Task, error) {
	var entity TaskEntity
	err := r.db.WithContext(ctx).
		Where(&TaskEntity{ChatID: chatID, Started: true, Done: false}).
		Order("priority ASC").
		Order("ignore_count DESC").
		Order("scheduled_at ASC").
		First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task := toTaskModel(entity)
	return &task, nil
}

func (r *SQLiteRepo) GetTaskByID(ctx context.Context, chatID, taskID int64) (*models.Task, error) {
	var entity TaskEntity
	err := r.db.WithContext(ctx).
		Where("chat_id = ? AND id = ?", chatID, taskID).
		First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task := toTaskModel(entity)
	return &task, nil
}

func (r *SQLiteRepo) StartTask(ctx context.Context, chatID, taskID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TaskEntity{}).
			Where(&TaskEntity{ChatID: chatID, Done: false}).
			Update("started", false).Error; err != nil {
			return err
		}

		res := tx.Model(&TaskEntity{}).
			Where("chat_id = ? AND id = ? AND done = ?", chatID, taskID, false).
			Update("started", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("task %d not found or already done", taskID)
		}
		return nil
	})
}

func (r *SQLiteRepo) StopActiveTask(ctx context.Context, chatID int64) error {
	return r.db.WithContext(ctx).
		Model(&TaskEntity{}).
		Where("chat_id = ? AND started = ? AND done = ?", chatID, true, false).
		Update("started", false).Error
}

func (r *SQLiteRepo) CompleteTask(ctx context.Context, chatID, taskID int64) error {
	res := r.db.WithContext(ctx).
		Model(&TaskEntity{}).
		Where("chat_id = ? AND id = ? AND started = ? AND done = ?", chatID, taskID, true, false).
		Updates(map[string]any{"done": true, "started": false})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("task %d is not started or not found", taskID)
	}
	return nil
}

func (r *SQLiteRepo) GetCandidateTasks(ctx context.Context, chatID int64) ([]models.Task, error) {
	var entities []TaskEntity
	if err := r.db.WithContext(ctx).
		Where(&TaskEntity{ChatID: chatID, Done: false}).
		Order("priority ASC").
		Order("ignore_count DESC").
		Order("scheduled_at ASC").
		Find(&entities).Error; err != nil {
		return nil, err
	}

	out := make([]models.Task, 0, len(entities))
	for _, entity := range entities {
		out = append(out, toTaskModel(entity))
	}
	return out, nil
}

func (r *SQLiteRepo) UpdateTaskReminder(ctx context.Context, chatID, taskID int64, remindedAt time.Time, newIgnoreCount int, newPriority int) error {
	return r.db.WithContext(ctx).
		Model(&TaskEntity{}).
		Where("chat_id = ? AND id = ?", chatID, taskID).
		Updates(map[string]any{
			"last_reminded_at": remindedAt.UTC(),
			"ignore_count":     newIgnoreCount,
			"priority":         newPriority,
		}).Error
}

func (r *SQLiteRepo) UpdateTaskSchedule(ctx context.Context, chatID, taskID int64, scheduledAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&TaskEntity{}).
		Where("chat_id = ? AND id = ?", chatID, taskID).
		Update("scheduled_at", scheduledAt.UTC()).Error
}

func (r *SQLiteRepo) IsBusyNow(ctx context.Context, chatID int64, now time.Time) (bool, error) {
	var openCount int64
	if err := r.db.WithContext(ctx).
		Model(&BusySlotEntity{}).
		Where("chat_id = ? AND is_open = ?", chatID, true).
		Count(&openCount).Error; err != nil {
		return false, err
	}
	if openCount > 0 {
		return true, nil
	}

	day := toDay(now.Weekday())
	cur := now.Format("15:04")

	var count int64
	err := r.db.WithContext(ctx).
		Model(&BusySlotEntity{}).
		Where("chat_id = ? AND day = ? AND is_open = ?", chatID, day, false).
		Where("start <= ? AND end >= ?", cur, cur).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteRepo) CurrentSlotRemainingMinutes(ctx context.Context, chatID int64, now time.Time) (int, error) {
	day := toDay(now.Weekday())
	cur := now.Format("15:04")

	var slots []FreeSlotEntity
	if err := r.db.WithContext(ctx).
		Where("chat_id = ? AND day = ?", chatID, day).
		Find(&slots).Error; err != nil {
		return 0, err
	}

	maxRemain := 0
	for _, slot := range slots {
		if !(slot.Start <= cur && cur <= slot.End) {
			continue
		}
		effectiveEnd := slot.End

		var busySlots []BusySlotEntity
		if err := r.db.WithContext(ctx).
			Where("chat_id = ? AND day = ? AND is_open = ?", chatID, day, false).
			Where("start >= ? AND start <= ?", cur, slot.End).
			Order("start ASC").
			Find(&busySlots).Error; err != nil {
			return 0, err
		}
		if len(busySlots) > 0 && busySlots[0].Start < effectiveEnd {
			effectiveEnd = busySlots[0].Start
		}

		endTime, err := time.ParseInLocation("15:04", effectiveEnd, now.Location())
		if err != nil {
			continue
		}
		finish := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())
		remain := int(finish.Sub(now).Minutes())
		if remain > maxRemain {
			maxRemain = remain
		}
	}
	return maxRemain, nil
}

func (r *SQLiteRepo) GetUserIDs(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&UserEntity{}).Pluck("chat_id", &ids).Error
	return ids, err
}

func (r *SQLiteRepo) NextFreeSlotStart(ctx context.Context, chatID int64, now time.Time) (time.Time, bool, error) {
	var slots []FreeSlotEntity
	if err := r.db.WithContext(ctx).Where(&FreeSlotEntity{ChatID: chatID}).Find(&slots).Error; err != nil {
		return time.Time{}, false, err
	}

	var (
		best    time.Time
		hasBest bool
	)
	for _, slot := range slots {
		cand, ok := nextWeekdayTime(now, slot.Day, slot.Start)
		if !ok {
			continue
		}
		if !hasBest || cand.Before(best) {
			best = cand
			hasBest = true
		}
	}
	return best, hasBest, nil
}

func toTaskModel(entity TaskEntity) models.Task {
	return models.Task{
		ID:             int64(entity.ID),
		ChatID:         entity.ChatID,
		Name:           entity.Name,
		Duration:       entity.Duration,
		Type:           entity.Type,
		Priority:       entity.Priority,
		ScheduledAt:    entity.ScheduledAt,
		Started:        entity.Started,
		Done:           entity.Done,
		IgnoreCount:    entity.IgnoreCount,
		LastRemindedAt: entity.LastRemindedAt,
	}
}

func toDay(w time.Weekday) int {
	if w == time.Sunday {
		return 7
	}
	return int(w)
}

func nextWeekdayTime(now time.Time, day int, hhmm string) (time.Time, bool) {
	tm, err := time.ParseInLocation("15:04", hhmm, now.Location())
	if err != nil {
		return time.Time{}, false
	}
	curDay := toDay(now.Weekday())
	delta := day - curDay
	if delta < 0 {
		delta += 7
	}
	cand := time.Date(now.Year(), now.Month(), now.Day(), tm.Hour(), tm.Minute(), 0, 0, now.Location()).AddDate(0, 0, delta)
	if cand.Before(now) || cand.Equal(now) {
		cand = cand.AddDate(0, 0, 7)
	}
	return cand, true
}

func (r *SQLiteRepo) DebugState(ctx context.Context, chatID int64) (string, error) {
	var freeCount, busyCount, taskCount, activeCount int64

	if err := r.db.WithContext(ctx).Model(&FreeSlotEntity{}).Where(&FreeSlotEntity{ChatID: chatID}).Count(&freeCount).Error; err != nil {
		return "", err
	}
	if err := r.db.WithContext(ctx).Model(&BusySlotEntity{}).Where(&BusySlotEntity{ChatID: chatID}).Count(&busyCount).Error; err != nil {
		return "", err
	}
	if err := r.db.WithContext(ctx).Model(&TaskEntity{}).Where("chat_id = ? AND done = ?", chatID, false).Count(&taskCount).Error; err != nil {
		return "", err
	}
	if err := r.db.WithContext(ctx).Model(&TaskEntity{}).Where("chat_id = ? AND started = ? AND done = ?", chatID, true, false).Count(&activeCount).Error; err != nil {
		return "", err
	}

	return fmt.Sprintf("free_slots=%d, busy_slots=%d, tasks_open=%d, active=%d", freeCount, busyCount, taskCount, activeCount), nil
}
