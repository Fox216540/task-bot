package bot

func tr(lang, key string) string {
	if lang != "en" {
		lang = "ru"
	}
	if val, ok := translations[lang][key]; ok {
		return val
	}
	return translations["ru"][key]
}

var translations = map[string]map[string]string{
	"ru": {
		"welcome":        "Привет. Я Telegram Task Scheduler Bot.\nДобавь свободные окна через /free и задачи через /task.",
		"usage_lang":     "Использование: /lang ru|en",
		"lang_set":       "Язык обновлен.",
		"usage_free":     "Использование: /free mon,wed 16:00-18:00",
		"free_saved":     "Свободные слоты сохранены.",
		"usage_busy":     "Использование: /busy today 16:00-18:00 или /busy now",
		"busy_saved":     "Период занятости сохранен.",
		"busy_now_saved": "Отмечено как занято прямо сейчас. Используй /unbusy чтобы снять блок.",
		"unbusy_done":    "Открытая занятость закрыта.",
		"usage_task":     "Использование: /task NAME | DURATION | TYPE | PRIORITY",
		"task_saved":     "Задача сохранена.",
		"bad_command":    "Не понял команду. /info",
		"info":           "Команды:\n/start\n/lang ru|en\n/info\n/free mon,wed 16:00-18:00\n/busy today 16:00-18:00\n/busy now\n/unbusy\n/task NAME | DURATION | TYPE | PRIORITY\n\nТипы: с/deep(15), м/medium(10), л/light(5), б/quick(2).",
		"hard_prompt":    "Сейчас есть время. Делай: %s\nДлительность: %d мин.",
		"start_ok":       "Задача запущена. Работай.",
		"done_ok":        "Задача завершена.",
		"start_required": "Сначала нажми 🚀 для старта задачи.",
		"no_task":        "Подходящей задачи сейчас нет.",
		"still_busy":     "Сейчас занят. Напоминания отключены временно.",
	},
	"en": {
		"welcome":        "Hi. I am Telegram Task Scheduler Bot.\nAdd free slots with /free and tasks with /task.",
		"usage_lang":     "Usage: /lang ru|en",
		"lang_set":       "Language updated.",
		"usage_free":     "Usage: /free mon,wed 16:00-18:00",
		"free_saved":     "Free slots saved.",
		"usage_busy":     "Usage: /busy today 16:00-18:00 or /busy now",
		"busy_saved":     "Busy period saved.",
		"busy_now_saved": "Marked as busy now. Use /unbusy to close it.",
		"unbusy_done":    "Open busy period closed.",
		"usage_task":     "Usage: /task NAME | DURATION | TYPE | PRIORITY",
		"task_saved":     "Task saved.",
		"bad_command":    "Unknown command. Try /info",
		"info":           "Commands:\n/start\n/lang ru|en\n/info\n/free mon,wed 16:00-18:00\n/busy today 16:00-18:00\n/busy now\n/unbusy\n/task NAME | DURATION | TYPE | PRIORITY\n\nTypes: c/deep(15), m/medium(10), l/light(5), b/quick(2).",
		"hard_prompt":    "You have free time now. Do: %s\nDuration: %d min.",
		"start_ok":       "Task started.",
		"done_ok":        "Task completed.",
		"start_required": "Press 🚀 to start the task first.",
		"no_task":        "No suitable task right now.",
		"still_busy":     "You are busy now. Reminders are paused.",
	},
}
