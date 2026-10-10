package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type skillInfo struct {
	name        string
	description string
	path        string
}

func validSkillName(name string) bool {
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

func discoverSkills(dir string) ([]skillInfo, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skills directory: %w", err)
	}

	var skills []skillInfo
	for _, entry := range entries {
		if !entry.IsDir() || !validSkillName(entry.Name()) {
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
		name, description, err := readSkillFrontmatter(bufio.NewReader(file))
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("parse skill %q: %w", entry.Name(), err)
		}
		if name != entry.Name() {
			return nil, fmt.Errorf("skill %q has mismatched name %q", entry.Name(), name)
		}
		skills = append(skills, skillInfo{name: name, description: description, path: path})
	}
	return skills, nil
}

func readSkillBody(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open skill body: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	if _, _, err := readSkillFrontmatter(reader); err != nil {
		return "", fmt.Errorf("parse skill body: %w", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read skill body: %w", err)
	}
	return strings.Trim(string(body), "\r\n"), nil
}

func readSkillFrontmatter(reader *bufio.Reader) (string, string, error) {
	first, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", "", err
	}
	if strings.TrimRight(first, "\r\n") != "---" {
		return "", "", fmt.Errorf("missing opening ---")
	}

	var name, description string
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "---" {
			if name == "" || description == "" {
				return "", "", fmt.Errorf("missing name or description")
			}
			return name, description, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") {
				value, err = strconv.Unquote(value)
				if err != nil {
					return "", "", fmt.Errorf("invalid quoted %s: %w", key, err)
				}
			} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
				value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
			}
			switch strings.TrimSpace(key) {
			case "name":
				name = value
			case "description":
				description = value
			}
		}
		if err == io.EOF {
			return "", "", fmt.Errorf("missing closing ---")
		}
	}
}
