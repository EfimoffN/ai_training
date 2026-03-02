package agent

import (
	"fmt"
	"strings"
)

// StickyFacts — стратегия с извлечением ключевых фактов.
// Хранит отдельный блок "facts" (обновляемый LLM) + последние N сообщений.
type StickyFacts struct {
	keepLast    int
	history     []Message
	facts       string // текущие извлечённые факты
	needExtract bool   // нужно ли извлекать факты после ответа
}

// NewStickyFacts создаёт стратегию. keepLast — сколько последних сообщений хранить.
func NewStickyFacts(keepLast int) *StickyFacts {
	return &StickyFacts{keepLast: keepLast}
}

func (s *StickyFacts) Name() string { return "Sticky Facts" }

func (s *StickyFacts) BuildSystem(base string) string {
	if s.facts == "" {
		return base
	}
	return base + "\n\n" +
		"ВАЖНЫЕ ФАКТЫ ИЗ ДИАЛОГА (используй их для ответа):\n" + s.facts
}

func (s *StickyFacts) BuildMessages() []Message {
	if len(s.history) <= s.keepLast {
		return s.history
	}
	return s.history[len(s.history)-s.keepLast:]
}

func (s *StickyFacts) AddUser(content string) {
	s.history = append(s.history, Message{Role: "user", Content: content})
	s.needExtract = true
}

func (s *StickyFacts) AddAssistant(content string) {
	s.history = append(s.history, Message{Role: "assistant", Content: content})
}

func (s *StickyFacts) PostProcess(llmCall func(string, []Message, int) (string, error)) {
	if !s.needExtract || len(s.history) < 2 {
		s.needExtract = false
		return
	}
	s.needExtract = false

	// Берём последнюю пару user+assistant для извлечения фактов
	recent := s.history[len(s.history)-2:]

	var sb strings.Builder
	if s.facts != "" {
		sb.WriteString("Текущие факты:\n")
		sb.WriteString(s.facts)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Новые сообщения:\n")
	for _, m := range recent {
		if m.Role == "user" {
			sb.WriteString("Пользователь: ")
		} else {
			sb.WriteString("Ассистент: ")
		}
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}

	system := "Ты — экстрактор фактов. " +
		"Обнови список ключевых фактов из диалога. " +
		"Формат: каждый факт на новой строке, в виде '- ключ: значение'. " +
		"Категории: цель, ограничения, предпочтения, решения, договорённости, имена, даты. " +
		"Удаляй устаревшие факты. Верни ТОЛЬКО список фактов."

	result, err := llmCall(system, []Message{{Role: "user", Content: sb.String()}}, 300)
	if err != nil {
		return // не критично — просто не обновим факты
	}

	s.facts = result
}

func (s *StickyFacts) Reset() {
	s.history = nil
	s.facts = ""
}

func (s *StickyFacts) Info() string {
	factsInfo := "пусто"
	if s.facts != "" {
		lines := strings.Count(s.facts, "\n") + 1
		factsInfo = fmt.Sprintf("%d фактов (%d символов)", lines, len(s.facts))
	}
	return fmt.Sprintf("KeepLast: %d | В истории: %d | Факты: %s",
		s.keepLast, len(s.history), factsInfo)
}

func (s *StickyFacts) HistoryLen() int {
	return len(s.history)
}

// GetFacts возвращает текущие извлечённые факты.
func (s *StickyFacts) GetFacts() string {
	return s.facts
}
