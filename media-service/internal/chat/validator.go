package chat

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrEmptyContent   = errors.New("chat content is empty")
	ErrContentTooLong = errors.New("chat content exceeds 1000 characters")
)

func ValidateContent(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", ErrEmptyContent
	}
	if utf8.RuneCountInString(trimmed) > 1000 {
		return "", ErrContentTooLong
	}
	return trimmed, nil
}
