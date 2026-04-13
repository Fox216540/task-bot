package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseFreeCommand(text string) ([]int, string, string, error) {
	return parseFreeCommand(text)
}

func ParseBusyCommand(text string, now time.Time) (days []int, start string, end string, isNow bool, err error) {
	return parseBusyCommand(text, now)
}

func ParseTaskCommand(text string) (name string, duration int, taskType string, priority int, err error) {
	return parseTaskCommand(text)
}

func parseLang(text string) (string, error) {
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return "", fmt.Errorf("usage")
	}
	lang := strings.ToLower(parts[1])
	if lang != "ru" && lang != "en" {
		return "", fmt.Errorf("usage")
	}
	return lang, nil
}

func parseFreeCommand(text string) ([]int, string, string, error) {
	parts := strings.Fields(text)
	if len(parts) != 3 {
		return nil, "", "", fmt.Errorf("usage")
	}
	days, err := parseDays(parts[1])
	if err != nil {
		return nil, "", "", err
	}
	start, end, err := parseRange(parts[2])
	if err != nil {
		return nil, "", "", err
	}
	return days, start, end, nil
}

func parseBusyCommand(text string, now time.Time) (days []int, start string, end string, isNow bool, err error) {
	parts := strings.Fields(text)
	if len(parts) == 2 && strings.ToLower(parts[1]) == "now" {
		return []int{dayNumber(now.Weekday())}, now.Format("15:04"), "23:59", true, nil
	}
	if len(parts) != 3 {
		return nil, "", "", false, fmt.Errorf("usage")
	}
	dayToken := strings.ToLower(parts[1])
	if dayToken == "today" || dayToken == "сегодня" {
		days = []int{dayNumber(now.Weekday())}
	} else {
		parsedDays, parseErr := parseDays(parts[1])
		if parseErr != nil || len(parsedDays) != 1 {
			return nil, "", "", false, fmt.Errorf("usage")
		}
		days = []int{parsedDays[0]}
	}
	start, end, err = parseRange(parts[2])
	if err != nil {
		return nil, "", "", false, err
	}
	return days, start, end, false, nil
}

func parseTaskCommand(text string) (name string, duration int, taskType string, priority int, err error) {
	raw := strings.TrimSpace(strings.TrimPrefix(text, "/task"))
	parts := strings.Split(raw, "|")
	if len(parts) != 4 {
		return "", 0, "", 0, fmt.Errorf("usage")
	}

	name = strings.TrimSpace(parts[0])
	if name == "" {
		return "", 0, "", 0, fmt.Errorf("usage")
	}

	duration, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || duration <= 0 {
		return "", 0, "", 0, fmt.Errorf("usage")
	}

	taskType = normalizeTaskType(strings.TrimSpace(parts[2]))
	if taskType == "" {
		return "", 0, "", 0, fmt.Errorf("usage")
	}

	priority, err = strconv.Atoi(strings.TrimSpace(parts[3]))
	if err != nil || priority < 1 {
		return "", 0, "", 0, fmt.Errorf("usage")
	}
	return name, duration, taskType, priority, nil
}

func parseDays(token string) ([]int, error) {
	rawDays := strings.Split(strings.ToLower(strings.TrimSpace(token)), ",")
	if len(rawDays) == 0 {
		return nil, fmt.Errorf("days")
	}
	seen := make(map[int]bool)
	var out []int
	for _, d := range rawDays {
		day, ok := dayAliases[strings.TrimSpace(d)]
		if !ok {
			return nil, fmt.Errorf("days")
		}
		if !seen[day] {
			seen[day] = true
			out = append(out, day)
		}
	}
	return out, nil
}

func parseRange(token string) (string, string, error) {
	parts := strings.Split(token, "-")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("range")
	}
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	if !isHHMM(start) || !isHHMM(end) || start >= end {
		return "", "", fmt.Errorf("range")
	}
	return start, end, nil
}

func isHHMM(v string) bool {
	_, err := time.Parse("15:04", v)
	return err == nil
}

func normalizeTaskType(raw string) string {
	switch strings.ToLower(raw) {
	case "с", "c", "deep":
		return "deep"
	case "м", "m", "medium":
		return "medium"
	case "л", "l", "light":
		return "light"
	case "б", "b", "quick":
		return "quick"
	default:
		return ""
	}
}

var dayAliases = map[string]int{
	"mon": 1, "monday": 1, "пн": 1, "пон": 1,
	"tue": 2, "tuesday": 2, "вт": 2, "вто": 2,
	"wed": 3, "wednesday": 3, "ср": 3, "сре": 3,
	"thu": 4, "thursday": 4, "чт": 4, "чет": 4,
	"fri": 5, "friday": 5, "пт": 5, "пят": 5,
	"sat": 6, "saturday": 6, "сб": 6, "суб": 6,
	"sun": 7, "sunday": 7, "вс": 7, "воск": 7,
}

func dayNumber(w time.Weekday) int {
	if w == time.Sunday {
		return 7
	}
	return int(w)
}
