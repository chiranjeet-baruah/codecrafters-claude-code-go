package domain

import (
	"regexp"
	"strconv"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Context     string
	Path        string
}

func ValidSkillName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if char != '-' && char != '_' && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

var skillPlaceholder = regexp.MustCompile(`\$(?:ARGUMENTS(?:\[[0-9]+\])?|[0-9]+)`)

func SubstituteSkillArguments(body, arguments string) string {
	arguments = strings.TrimSpace(arguments)
	args := strings.Fields(arguments)
	return skillPlaceholder.ReplaceAllStringFunc(body, func(placeholder string) string {
		if placeholder == "$ARGUMENTS" {
			return arguments
		}
		indexText := strings.TrimPrefix(placeholder, "$")
		if strings.HasPrefix(placeholder, "$ARGUMENTS[") {
			indexText = strings.TrimSuffix(strings.TrimPrefix(placeholder, "$ARGUMENTS["), "]")
		}
		index, err := strconv.Atoi(indexText)
		if err != nil || index >= len(args) {
			return ""
		}
		return args[index]
	})
}
