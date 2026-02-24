package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://api.anthropic.com/v1/messages"

// Agent — инкапсулирует логику общения с LLM.
// Хранит конфигурацию, историю сообщений и системный промпт.
type Agent struct {
	apiKey  string
	model   string
	system  string
	history []message
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type apiResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// New создаёт нового агента с заданным API-ключом, моделью и системным промптом.
func New(apiKey, model, system string) *Agent {
	return &Agent{
		apiKey: apiKey,
		model:  model,
		system: system,
	}
}

// Ask отправляет сообщение пользователя в LLM и возвращает ответ.
// История сообщений сохраняется — агент помнит контекст диалога.
func (a *Agent) Ask(userMessage string) (string, error) {
	a.history = append(a.history, message{Role: "user", Content: userMessage})

	body, err := json.Marshal(apiRequest{
		Model:     a.model,
		MaxTokens: 1024,
		System:    a.system,
		Messages:  a.history,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}

	text := result.Content[0].Text

	// Сохраняем ответ ассистента в историю для контекста
	a.history = append(a.history, message{Role: "assistant", Content: text})

	return text, nil
}

// Reset очищает историю диалога.
func (a *Agent) Reset() {
	a.history = nil
}

// HistoryLen возвращает количество сообщений в истории.
func (a *Agent) HistoryLen() int {
	return len(a.history)
}
