package server

import (
	"fmt"
	"net/http"
	"strings"
)

// Localization of server-side strings. Language: the X-Polka-Lang header
// (set by the frontend), otherwise the browser's/reader's Accept-Language.

func reqLang(r *http.Request) string {
	if l := r.Header.Get("X-Polka-Lang"); l == "en" || l == "ru" {
		return l
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "ru") {
		return "ru"
	}
	return "en"
}

var serverStrings = map[string]map[string]string{
	"ru": {
		"send.body":        "Книга отправлена сервером Polka",
		"send.noLibrary":   "Не настроен каталог с файлами книг",
		"send.smtpFail":    "Не удалось отправить письмо через SMTP",
		"send.invalid":     "Некорректный формат отправки",
		"send.busy":        "Отправка занята, попробуйте позже",
		"send.unavailable": "Конвертация недоступна для этой книги или отключена администратором",
		"send.large":       "Пустой файл или превышен допустимый размер книги",
		"send.convertFail": "Не удалось преобразовать FB2 в EPUB3",

		"opds.wishlist.sub":    "Книги из вашего списка «Хочу прочитать»",
		"shelf.reading":        "Читаю сейчас",
		"shelf.wishlist":       "Хочу прочитать",
		"shelf.offline":        "Доступно офлайн",
		"shelf.series_next":    "Продолжить серии",
		"shelf.for_you":        "Для вас",
		"shelf.collection.sub": "В библиотеке %d из %d",
		"collection.offline":   "Офлайн-библиотека",
		"reader.start":         "Начало",
		"reader.chapter":       "Глава %d",
		"opds.new":             "Новинки",
		"opds.new.sub":         "Недавно добавленные книги",
		"opds.authors":         "По авторам",
		"opds.authors.sub":     "Книги по авторам, по алфавиту",
		"opds.series":          "По сериям",
		"opds.series.sub":      "Книги по сериям, по алфавиту",
		"opds.genres":          "По жанрам",
		"opds.genres.sub":      "Книги по жанрам",
		"opds.reading":         "Читаю сейчас",
		"opds.reading.sub":     "Книги с сохранённой позицией чтения",
		"opds.search":          "Поиск: %s",
		"opds.books":           "%d книг",
		"upload.badformat":     "неподдерживаемый формат",
		"upload.readfail":      "не удалось прочитать файл",
		"upload.savefail":      "не удалось сохранить файл",
		"upload.dbfail":        "не удалось добавить в базу",
		"upload.dup.file":      "точная копия уже в библиотеке",
		"upload.dup.text":      "книга с тем же текстом уже в библиотеке",
	},
	"en": {
		"send.body":        "Book sent by the Polka server",
		"send.noLibrary":   "The book library directory is not configured",
		"send.smtpFail":    "Could not send the email through SMTP",
		"send.invalid":     "Invalid delivery format",
		"send.busy":        "Delivery is busy; try again later",
		"send.unavailable": "Conversion is unavailable for this book or disabled by the administrator",
		"send.large":       "The file is empty or exceeds the book size limit",
		"send.convertFail": "Could not convert FB2 to EPUB3",

		"opds.wishlist.sub":    "Books from your personal Want to read list",
		"shelf.reading":        "Reading now",
		"shelf.wishlist":       "Want to read",
		"shelf.offline":        "Available offline",
		"shelf.series_next":    "Continue the series",
		"shelf.for_you":        "For you",
		"shelf.collection.sub": "%d of %d in the library",
		"collection.offline":   "Offline library",
		"reader.start":         "Beginning",
		"reader.chapter":       "Chapter %d",
		"opds.new":             "New books",
		"opds.new.sub":         "Recently added books",
		"opds.authors":         "By author",
		"opds.authors.sub":     "Books by author, alphabetical",
		"opds.series":          "By series",
		"opds.series.sub":      "Books by series, alphabetical",
		"opds.genres":          "By genre",
		"opds.genres.sub":      "Books by genre",
		"opds.reading":         "Reading now",
		"opds.reading.sub":     "Books with saved reading position",
		"opds.search":          "Search: %s",
		"opds.books":           "%d books",
		"upload.badformat":     "unsupported format",
		"upload.readfail":      "failed to read the file",
		"upload.savefail":      "failed to save the file",
		"upload.dbfail":        "failed to add to the database",
		"upload.dup.file":      "exact copy is already in the library",
		"upload.dup.text":      "a book with the same text is already in the library",
	},
}

func tr(lang, key string, args ...any) string {
	s, ok := serverStrings[lang][key]
	if !ok {
		s = serverStrings["ru"][key]
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}
