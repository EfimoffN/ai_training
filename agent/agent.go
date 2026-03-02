package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://api.anthropic.com/v1/messages"

// Message — сообщение в диалоге (экспортируемый тип для стратегий).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
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

// Stats — накопленная статистика по токенам.
type Stats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCost         float64
	Requests          int

	LastInputTokens  int
	LastOutputTokens int
	LastCost         float64
}

const (
	inputPricePer1M  = 0.80
	outputPricePer1M = 4.00
)

// Strategy — интерфейс стратегии управления контекстом.
type Strategy interface {
	// Name возвращает название стратегии.
	Name() string
	// BuildSystem формирует системный промпт (может добавлять facts и т.д.).
	BuildSystem(baseSystem string) string
	// BuildMessages возвращает сообщения для отправки в API.
	BuildMessages() []Message
	// AddUser добавляет сообщение пользователя.
	AddUser(content string)
	// AddAssistant добавляет ответ ассистента.
	AddAssistant(content string)
	// PostProcess вызывается после получения ответа (для извлечения фактов и т.п.).
	// Получает доступ к LLM через callback.
	PostProcess(llmCall func(system string, msgs []Message, maxTokens int) (string, error))
	// Reset очищает всё состояние.
	Reset()
	// Info возвращает текстовую информацию о текущем состоянии стратегии.
	Info() string
	// HistoryLen возвращает количество сообщений.
	HistoryLen() int
}

// Agent — инкапсулирует LLM-вызовы и делегирует управление контекстом стратегии.
type Agent struct {
	apiKey   string
	model    string
	system   string
	strategy Strategy
	stats    Stats
}

// New создаёт агента с заданной стратегией.
func New(apiKey, model, system string, strategy Strategy) *Agent {
	return &Agent{
		apiKey:   apiKey,
		model:    model,
		system:   system,
		strategy: strategy,
	}
}

// Ask отправляет сообщение пользователя и возвращает ответ.
func (a *Agent) Ask(userMessage string) (string, error) {
	a.strategy.AddUser(userMessage)

	msgs := a.strategy.BuildMessages()
	sys := a.strategy.BuildSystem(a.system)

	body, err := json.Marshal(apiRequest{
		Model:     a.model,
		MaxTokens: 1024,
		System:    sys,
		Messages:  msgs,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	text, usage, err := a.doRequest(body)
	if err != nil {
		return "", err
	}

	a.updateStats(usage.input, usage.output)
	a.strategy.AddAssistant(text)

	// PostProcess — стратегия может вызвать LLM (например, для извлечения фактов)
	a.strategy.PostProcess(func(sys string, msgs []Message, maxTokens int) (string, error) {
		b, err := json.Marshal(apiRequest{
			Model:     a.model,
			MaxTokens: maxTokens,
			System:    sys,
			Messages:  msgs,
		})
		if err != nil {
			return "", err
		}
		text, usage, err := a.doRequest(b)
		if err != nil {
			return "", err
		}
		a.updateStats(usage.input, usage.output)
		return text, nil
	})

	return text, nil
}

// SetStrategy переключает стратегию на лету.
func (a *Agent) SetStrategy(s Strategy) {
	a.strategy = s
	a.stats = Stats{} // сбрасываем статистику при смене стратегии
}

// GetStrategy возвращает текущую стратегию.
func (a *Agent) GetStrategy() Strategy {
	return a.strategy
}

// GetStats возвращает статистику.
func (a *Agent) GetStats() Stats {
	return a.stats
}

// Reset сбрасывает стратегию и статистику.
func (a *Agent) Reset() {
	a.strategy.Reset()
	a.stats = Stats{}
}

// HistoryLen делегирует стратегии.
func (a *Agent) HistoryLen() int {
	return a.strategy.HistoryLen()
}

type usageInfo struct {
	input  int
	output int
}

func (a *Agent) doRequest(body []byte) (string, usageInfo, error) {
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", usageInfo{}, err
	}

	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", usageInfo{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", usageInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", usageInfo{}, fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", usageInfo{}, err
	}

	if len(result.Content) == 0 {
		return "", usageInfo{}, fmt.Errorf("empty response")
	}

	return result.Content[0].Text, usageInfo{
		input:  result.Usage.InputTokens,
		output: result.Usage.OutputTokens,
	}, nil
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
