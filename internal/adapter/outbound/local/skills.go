package local

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

func DiscoverSkills(dir string) ([]domain.Skill, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skills directory: %w", err)
	}

	var skills []domain.Skill
	for _, entry := range entries {
		if !entry.IsDir() || !domain.ValidSkillName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "SKILL.md")
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("open skill %q: %w", entry.Name(), err)
		}
		name, description, context, err := readSkillFrontmatter(bufio.NewReader(file))
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("parse skill %q: %w", entry.Name(), err)
		}
		if name != entry.Name() {
			return nil, fmt.Errorf("skill %q has mismatched name %q", entry.Name(), name)
		}
		skills = append(skills, domain.Skill{Name: name, Description: description, Context: context, Path: path})
	}
	return skills, nil
}

type SkillStore struct{}

func (SkillStore) ReadBody(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open skill body: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	if _, _, _, err := readSkillFrontmatter(reader); err != nil {
		return "", fmt.Errorf("parse skill body: %w", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read skill body: %w", err)
	}
	return strings.Trim(string(body), "\r\n"), nil
}

func readSkillFrontmatter(reader *bufio.Reader) (string, string, string, error) {
	first, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", "", "", err
	}
	if strings.TrimRight(first, "\r\n") != "---" {
		return "", "", "", fmt.Errorf("missing opening ---")
	}

	var name, description, context string
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", "", "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "---" {
			if name == "" || description == "" {
				return "", "", "", fmt.Errorf("missing name or description")
			}
			return name, description, context, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") {
				value, err = strconv.Unquote(value)
				if err != nil {
					return "", "", "", fmt.Errorf("invalid quoted %s: %w", key, err)
				}
			} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
				value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
			}
			switch strings.TrimSpace(key) {
			case "name":
				name = value
			case "description":
				description = value
			case "context":
				context = value
			}
		}
		if err == io.EOF {
			return "", "", "", fmt.Errorf("missing closing ---")
		}
	}
}
