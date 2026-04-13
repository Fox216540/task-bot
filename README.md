# Telegram Task Scheduler Bot (Full Spec — Hard Mode + Adaptive + SQLite)

## Overview

Telegram-бот с **жёстким управлением задачами**:

* авто-планирование по свободным окнам
* динамический `/busy` (включая бесконечный `/busy now`)
* буфер между задачами
* типы задач (влияют на буфер)
* приоритеты
* **Hard Mode (навязывание задачи)**
* **Escalation (чаще напоминания при игноре)**
* **Adaptive Behavior (игнор → ↑ приоритет)**
* **Smart Selection (под слот)**
* SQLite хранение
* RU / EN

---

## Core Principle

> если ты свободен → система заставляет тебя начать задачу

---

## Stack

* Python 3.11+
* python-telegram-bot v20+
* APScheduler
* SQLite

---

## Database Schema

```sql
CREATE TABLE users (
    chat_id INTEGER PRIMARY KEY,
    lang TEXT DEFAULT 'ru'
);

CREATE TABLE free_slots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER,
    day INTEGER,
    start TEXT,
    end TEXT
);

CREATE TABLE busy_slots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER,
    day INTEGER,
    start TEXT,
    end TEXT,
    is_open INTEGER DEFAULT 0
);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER,
    name TEXT,
    duration INTEGER,
    type TEXT,
    priority INTEGER,
    scheduled_at TEXT,
    started INTEGER DEFAULT 0,
    done INTEGER DEFAULT 0,
    ignore_count INTEGER DEFAULT 0,
    last_reminded_at TEXT
);
```

---

## Commands

### `/start`

Регистрация

### `/lang ru|en`

Язык

### `/info`

Инструкция + состояние

---

### `/free`

```
/free mon,wed 16:00-18:00
/free пн 20:00-22:00
```

---

### `/busy`

```
/busy today 16:00-18:00
/busy mon 14:00-18:00
/busy now
```

---

### `/unbusy`

```
/unbusy
```

---

### `/task`

```
/task NAME | DURATION | TYPE | PRIORITY
```

Пример:

```
/task Сделать API | 30 | с | 1
```

---

## Task Types

| Код | Тип    | Буфер |
| --- | ------ | ----- |
| с   | deep   | 15    |
| м   | medium | 10    |
| л   | light  | 5     |
| б   | quick  | 2     |

---

## Busy Logic

### Open busy

```
/busy now → is_open = 1
/unbusy → закрывает
```

---

### Проверка

```
IF (start <= now <= end)
OR (is_open AND start <= now)
→ busy
```

---

## Hard Mode

### Поведение

```
Сейчас есть время. Делай: <name>

[🚀]
[✅]
```

---

### Нет:

* пропуска
* выбора другой задачи

---

## Reminder Loop

```
IF NOT started
AND NOT done
AND NOT busy
→ напоминание
```

---

## Escalation

| ignore | интервал |
| ------ | -------- |
| 0–4    | 3 мин    |
| 5–9    | 2 мин    |
| 10+    | 1 мин    |

---

## Adaptive Behavior (Pressure Mode)

```
ignore ↑ → приоритет ↑
```

---

### Сортировка

```
ORDER BY:
    priority ASC,
    ignore_count DESC,
    scheduled_at ASC
```

---

## Forced Tasks

```
ignore_count >= threshold → forced
```

---

### Ограничение (ВАЖНО)

```
task_total <= slot_duration
```

---

### Если НЕ помещается

```
forced игнорируется
→ выбрать другую задачу
```

---

## Smart Selection

### Алгоритм

1. Получить слот
2. Найти задачи:

```
duration + buffer <= slot
```

---

### Если есть

```
выбрать:
priority ASC
ignore DESC
```

---

### Если нет

```
взять самую короткую
```

---

## Dynamic Switching

```
если active_task не помещается:
    заменить
```

---

## Reschedule Logic

```
IF scheduled_at <= now:

    IF busy:
        → в следующий слот
        → не уведомлять

    ELSE:
        → сейчас
        → уведомить
```

---

## Suggestion Integration

```
/unbusy → сразу выбрать задачу
overdue → выбрать задачу
```

---

## Active Task Rule

```
одновременно только 1 задача
```

---

## Anti-Ignore

```
невозможно:
- пропустить
- выбрать другую
```

---

## Scheduler

* send task
* reminder loop (dynamic interval)

---

## Edge Cases

* overlapping busy
* нет слотов
* forced не помещается
* много задач

---

## Expected Behavior

| Сценарий             | Поведение           |
| -------------------- | ------------------- |
| свободен             | задача навязывается |
| игноришь             | чаще напоминания    |
| занят                | тишина              |
| освободился          | сразу задача        |
| задача не помещается | замена              |

---

## Final Goal

> система не даёт игнорировать задачи и заставляет начать действие

---

## Go Implementation Notes

Эта версия проекта реализована на Go + SQLite через `gorm` и Telegram Go SDK (вместо Python-стека из исходного ТЗ), с разделением по логическим файлам:

- `cmd/task-bot/main.go` — запуск бота и reminder loop
- `internal/bot/*` — Telegram API, команды, i18n, hard-mode логика
- `internal/storage/sqlite.go` — SQLite схема и доступ к данным
- `internal/models/task.go` — модель задачи и буферы по типам
- `internal/config/config.go` — загрузка `BOT_TOKEN` и `DB_PATH`

### Run

```bash
export BOT_TOKEN="your_telegram_bot_token"
export DB_PATH="./task_bot.sqlite" # optional

go mod tidy
go run ./cmd/task-bot
```

### Tests

```bash
go test ./...
```
