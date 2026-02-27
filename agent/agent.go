package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const apiURL = "https://api.anthropic.com/v1/messages"

// Agent — инкапсулирует логику общения с LLM.
type Agent struct {
	apiKey      string
	model       string
	system      string
	history     []message  // полная история (для сохранения на диск)
	summary     string     // сжатый контекст старых сообщений
	keepLast    int        // сколько последних сообщений оставлять "как есть"
	compressAt  int        // при каком размере истории запускать сжатие
	historyFile string
	stats       Stats
	Compressed  bool // было ли сжатие в последнем запросе
}

// Stats — накопленная статистика по токенам за сессию.
type Stats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCost         float64
	Requests          int

	LastInputTokens  int
	LastOutputTokens int
	LastCost         float64

	Compressions int // сколько раз выполнялось сжатие
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

const (
	inputPricePer1M  = 0.80
	outputPricePer1M = 4.00
)

// persistData — структура для сохранения на диск (история + summary).
type persistData struct {
	Summary string    `json:"summary,omitempty"`
	History []message `json:"history"`
}

// New создаёт нового агента.
// keepLast — сколько последних сообщений сохранять без сжатия.
// compressAt — при каком количестве сообщений запускать сжатие.
func New(apiKey, model, system, historyFile string, keepLast, compressAt int) *Agent {
	a := &Agent{
		apiKey:      apiKey,
		model:       model,
		system:      system,
		historyFile: historyFile,
		keepLast:    keepLast,
		compressAt:  compressAt,
	}

	if historyFile != "" {
		a.load()
	}

	return a
}

// Ask отправляет сообщение пользователя в LLM и возвращает ответ.
func (a *Agent) Ask(userMessage string) (string, error) {
	a.history = append(a.history, message{Role: "user", Content: userMessage})
	a.Compressed = false

	// Сжимаем историю, если она превысила порог
	if len(a.history) > a.compressAt {
		if err := a.compress(); err != nil {
			return "", fmt.Errorf("compress: %w", err)
		}
	}

	// Собираем сообщения для API: summary + последние N
	msgs := a.buildMessages()

	body, err := json.Marshal(apiRequest{
		Model:     a.model,
		MaxTokens: 1024,
		System:    a.buildSystem(),
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
	a.history = append(a.history, message{Role: "assistant", Content: text})
	a.save()

	return text, nil
}

// GetStats возвращает текущую статистику.
func (a *Agent) GetStats() Stats {
	return a.stats
}

// GetSummary возвращает текущий сжатый контекст.
func (a *Agent) GetSummary() string {
	return a.summary
}

// Reset очищает всё.
func (a *Agent) Reset() {
	a.history = nil
	a.summary = ""
	a.stats = Stats{}
	if a.historyFile != "" {
		os.Remove(a.historyFile)
	}
}

// HistoryLen возвращает количество сообщений в текущей истории.
func (a *Agent) HistoryLen() int {
	return len(a.history)
}

// buildSystem формирует системный промпт с учётом summary.
func (a *Agent) buildSystem() string {
	if a.summary == "" {
		return a.system
	}
	return a.system + "\n\n" +
		"Краткое содержание предыдущей части диалога:\n" + a.summary
}

// buildMessages возвращает сообщения для отправки в API.
func (a *Agent) buildMessages() []message {
	return a.history
}

// compress сжимает старые сообщения в summary через вызов LLM.
func (a *Agent) compress() error {
	// Определяем, какие сообщения сжать (всё кроме последних keepLast)
	cutoff := len(a.history) - a.keepLast
	if cutoff <= 0 {
		return nil
	}

	oldMessages := a.history[:cutoff]

	// Формируем текст для суммаризации
	var sb strings.Builder
	if a.summary != "" {
		sb.WriteString("Предыдущее краткое содержание:\n")
		sb.WriteString(a.summary)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Новые сообщения для суммаризации:\n")
	for _, m := range oldMessages {
		if m.Role == "user" {
			sb.WriteString("Пользователь: ")
		} else {
			sb.WriteString("Ассистент: ")
		}
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}

	// Просим LLM сжать
	body, err := json.Marshal(apiRequest{
		Model:     a.model,
		MaxTokens: 300,
		System: "Ты — суммаризатор диалогов. " +
			"Сожми диалог в краткое содержание (3-5 предложений). " +
			"Сохрани ключевые факты, имена и решения. " +
			"Верни ТОЛЬКО краткое содержание.",
		Messages: []message{{Role: "user", Content: sb.String()}},
	})
	if err != nil {
		return err
	}

	text, usage, err := a.doRequest(body)
	if err != nil {
		return err
	}

	a.updateStats(usage.input, usage.output)

	// Обновляем summary и обрезаем историю
	a.summary = text
	a.history = a.history[cutoff:]
	a.stats.Compressions++
	a.Compressed = true

	return nil
}

type usageInfo struct {
	input  int
	output int
}

func (a *Agent) doRequest(body []byte) (string, usageInfo, error) {
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", usageInfo{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", usageInfo{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", usageInfo{}, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", usageInfo{}, fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", usageInfo{}, fmt.Errorf("unmarshal: %w", err)
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

func (a *Agent) load() {
	data, err := os.ReadFile(a.historyFile)
	if err != nil {
		return
	}

	var pd persistData
	if err := json.Unmarshal(data, &pd); err != nil {
		// Обратная совместимость: пробуем старый формат (просто массив)
		var msgs []message
		if err := json.Unmarshal(data, &msgs); err != nil {
			return
		}
		a.history = msgs
		return
	}

	a.summary = pd.Summary
	a.history = pd.History
}

func (a *Agent) save() {
	if a.historyFile == "" {
		return
	}

	data, err := json.MarshalIndent(persistData{
		Summary: a.summary,
		History: a.history,
	}, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(a.historyFile, data, 0644)
}
