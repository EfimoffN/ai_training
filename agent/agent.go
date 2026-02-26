package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const apiURL = "https://api.anthropic.com/v1/messages"

// Agent — инкапсулирует логику общения с LLM.
type Agent struct {
	apiKey      string
	model       string
	system      string
	history     []message
	historyFile string
	stats       Stats
}

// Stats — накопленная статистика по токенам за сессию.
type Stats struct {
	TotalInputTokens  int     // сколько всего input-токенов отправлено
	TotalOutputTokens int     // сколько всего output-токенов получено
	TotalCost         float64 // суммарная стоимость в USD
	Requests          int     // количество запросов

	// Последний запрос
	LastInputTokens  int
	LastOutputTokens int
	LastCost         float64
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

// Цены за 1M токенов (USD) для Haiku 3.5
const (
	inputPricePer1M  = 0.80
	outputPricePer1M = 4.00
)

// New создаёт нового агента.
func New(apiKey, model, system, historyFile string) *Agent {
	a := &Agent{
		apiKey:      apiKey,
		model:       model,
		system:      system,
		historyFile: historyFile,
	}

	if historyFile != "" {
		a.load()
	}

	return a
}

// Ask отправляет сообщение пользователя в LLM и возвращает ответ.
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

	// Обновляем статистику
	a.updateStats(result.Usage.InputTokens, result.Usage.OutputTokens)

	a.history = append(a.history, message{Role: "assistant", Content: text})
	a.save()

	return text, nil
}

// GetStats возвращает текущую статистику.
func (a *Agent) GetStats() Stats {
	return a.stats
}

// Reset очищает историю диалога, файл и статистику.
func (a *Agent) Reset() {
	a.history = nil
	a.stats = Stats{}
	if a.historyFile != "" {
		os.Remove(a.historyFile)
	}
}

// HistoryLen возвращает количество сообщений в истории.
func (a *Agent) HistoryLen() int {
	return len(a.history)
}

func (a *Agent) updateStats(inputTokens, outputTokens int) {
	cost := float64(inputTokens)/1_000_000*inputPricePer1M +
		float64(outputTokens)/1_000_000*outputPricePer1M

	a.stats.LastInputTokens = inputTokens
	a.stats.LastOutputTokens = outputTokens
	a.stats.LastCost = cost

	a.stats.TotalInputTokens += inputTokens
	a.stats.TotalOutputTokens += outputTokens
	a.stats.TotalCost += cost
	a.stats.Requests++
}

func (a *Agent) load() {
	data, err := os.ReadFile(a.historyFile)
	if err != nil {
		return
	}

	var msgs []message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return
	}

	a.history = msgs
}

func (a *Agent) save() {
	if a.historyFile == "" {
		return
	}

	data, err := json.MarshalIndent(a.history, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(a.historyFile, data, 0644)
}
