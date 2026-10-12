package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

type testMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func TestNewClientAndComplete(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		response   string
		wantError  string
		wantResult string
	}{
		{"success", http.StatusOK, `{"id":"chat_1","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`, "", "ok"},
		{"no choices", http.StatusOK, `{"id":"chat_1","object":"chat.completion","created":1,"model":"test","choices":[]}`, "no choices in response", ""},
		{"API error", http.StatusBadRequest, `{"error":{"message":"bad request","type":"invalid_request_error"}}`, "bad request", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("request path = %q, authorization = %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				var request struct {
					Model    string        `json:"model"`
					Messages []testMessage `json:"messages"`
					Tools    []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if request.Model != "anthropic/claude-haiku-4.5" || len(request.Messages) != 1 || request.Messages[0].Content != "hello" {
					t.Errorf("unexpected request: %+v", request)
				}
				var tools []string
				for _, tool := range request.Tools {
					tools = append(tools, tool.Function.Name)
				}
				if !reflect.DeepEqual(tools, []string{"Read", "Write", "Bash", "Skill"}) {
					t.Errorf("tools = %v, want Read, Write, Bash, Skill", tools)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.response)
			}))
			defer server.Close()
			t.Setenv("OPENROUTER_API_KEY", "test-key")
			t.Setenv("OPENROUTER_BASE_URL", server.URL)
			response, err := NewFromEnv().Complete(context.Background(), []domain.Message{{Role: "user", Content: "hello"}})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantError)
				}
			} else if err != nil || response.Content != tt.wantResult {
				t.Fatalf("response = %+v, error = %v; want %q", response, err, tt.wantResult)
			}
			if requests.Load() != 1 {
				t.Fatalf("requests = %d, want 1", requests.Load())
			}
		})
	}
}

func TestNewClientConfiguration(t *testing.T) {
	tests := []struct {
		name, key, baseURL string
		wantPanic          bool
	}{
		{"missing API key", "", "", true},
		{"default base URL", "test-key", "", false},
		{"custom base URL", "test-key", "https://example.test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENROUTER_API_KEY", tt.key)
			t.Setenv("OPENROUTER_BASE_URL", tt.baseURL)
			if tt.wantPanic {
				defer func() {
					if recover() == nil {
						t.Fatal("expected missing API key to panic")
					}
				}()
			}
			if client := NewFromEnv(); client == nil {
				t.Fatal("newClient returned nil")
			}
		})
	}
}
