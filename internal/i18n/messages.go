package i18n

import (
	"net/http"
	"strings"
)

const (
	LangRU = "ru"
	LangEN = "en"
)

var catalog = map[string]map[string]string{
	LangRU: {
		"xray process not running":                      "Процесс Xray не запущен",
		"xray running but VPN probe failed":             "Xray работает, но VPN probe не прошёл",
		"config and iptables applied, xray restarted":   "Config и iptables применены, Xray перезапущен",
		"selection saved":                               "Выбор сохранён",
		"unauthorized":                                  "Не авторизован",
		"url required":                                  "URL обязателен",
		"invalid current password":                      "Неверный текущий пароль",
		"password required":                             "Пароль обязателен",
		"onboarding required":                           "Требуется onboarding",
		"node not found":                                "Сервер не найден",
		"setup completed, all checks passed":            "Установка завершена, все проверки пройдены",
		"setup finished with warnings — check paths (USB must be mounted with xray)": "Установка завершена с предупреждениями — проверьте пути (USB с xray должен быть смонтирован)",
	},
	LangEN: {
		"xray process not running":                      "Xray process is not running",
		"xray running but VPN probe failed":             "Xray is running but VPN probe failed",
		"config and iptables applied, xray restarted":   "Config and iptables applied, Xray restarted",
		"selection saved":                               "Selection saved",
		"unauthorized":                                  "Unauthorized",
		"url required":                                  "URL is required",
		"invalid current password":                      "Invalid current password",
		"password required":                             "Password is required",
		"onboarding required":                           "Onboarding required",
		"node not found":                                "Server not found",
		"setup completed, all checks passed":            "Setup completed, all checks passed",
		"setup finished with warnings — check paths (USB must be mounted with xray)": "Setup finished with warnings — check paths (USB must be mounted with xray)",
	},
}

func LocaleFromRequest(r *http.Request) string {
	lang := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept-Language")))
	if strings.HasPrefix(lang, "en") {
		return LangEN
	}
	return LangRU
}

func T(locale, msg string) string {
	if locale == "" {
		locale = LangRU
	}
	if m, ok := catalog[locale][msg]; ok {
		return m
	}
	return msg
}

func LocalizeStatusMessage(locale, msg string) string {
	return T(locale, msg)
}
