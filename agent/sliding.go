package agent

import "fmt"

// SlidingWindow — стратегия скользящего окна.
// Хранит только последние N сообщений, остальное отбрасывает.
type SlidingWindow struct {
	windowSize int
	history    []Message
	dropped    int // сколько сообщений было отброшено
}

// NewSlidingWindow создаёт стратегию с окном размера n.
func NewSlidingWindow(n int) *SlidingWindow {
	return &SlidingWindow{windowSize: n}
}

func (s *SlidingWindow) Name() string { return "Sliding Window" }

func (s *SlidingWindow) BuildSystem(base string) string { return base }

func (s *SlidingWindow) BuildMessages() []Message {
	if len(s.history) <= s.windowSize {
		return s.history
	}
	return s.history[len(s.history)-s.windowSize:]
}

func (s *SlidingWindow) AddUser(content string) {
	s.history = append(s.history, Message{Role: "user", Content: content})
	s.trim()
}

func (s *SlidingWindow) AddAssistant(content string) {
	s.history = append(s.history, Message{Role: "assistant", Content: content})
	s.trim()
}

func (s *SlidingWindow) PostProcess(llmCall func(string, []Message, int) (string, error)) {
	// Sliding Window не использует LLM для пост-обработки
}

func (s *SlidingWindow) Reset() {
	s.history = nil
	s.dropped = 0
}

func (s *SlidingWindow) Info() string {
	return fmt.Sprintf("Окно: %d | В истории: %d | Отброшено: %d",
		s.windowSize, len(s.history), s.dropped)
}

func (s *SlidingWindow) HistoryLen() int {
	return len(s.history)
}

func (s *SlidingWindow) trim() {
	if len(s.history) > s.windowSize {
		excess := len(s.history) - s.windowSize
		s.dropped += excess
		s.history = s.history[excess:]
	}
}
