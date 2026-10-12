package main

import (
	"flag"
	"io"
	"os"
	"testing"
)

func TestParsePrompt(t *testing.T) {
	tests := []struct {
		name, prompt string
		wantPanic    bool
	}{
		{"ordinary", "hello", false},
		{"slash command", "/apple", false},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalFlags, originalArgs := flag.CommandLine, os.Args
			t.Cleanup(func() { flag.CommandLine, os.Args = originalFlags, originalArgs })
			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"program", "-p", tt.prompt}
			if tt.wantPanic {
				defer func() {
					if recover() == nil {
						t.Fatal("expected empty prompt to panic")
					}
				}()
			}
			if got := parsePrompt(); got != tt.prompt {
				t.Fatalf("prompt = %q, want %q", got, tt.prompt)
			}
		})
	}
}
