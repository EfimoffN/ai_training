package agent

import "fmt"

// Branch — именованная ветка диалога.
type Branch struct {
	Name    string
	History []Message
}

// Branching — стратегия с ветками диалога.
// Позволяет создавать checkpoint и продолжать диалог в разных направлениях.
type Branching struct {
	branches map[string]*Branch
	current  string // имя текущей ветки
}

// NewBranching создаёт стратегию с начальной веткой "main".
func NewBranching() *Branching {
	b := &Branching{
		branches: make(map[string]*Branch),
		current:  "main",
	}
	b.branches["main"] = &Branch{Name: "main"}
	return b
}

func (b *Branching) Name() string { return "Branching" }

func (b *Branching) BuildSystem(base string) string {
	return base + fmt.Sprintf("\n\n[Текущая ветка: %s]", b.current)
}

func (b *Branching) BuildMessages() []Message {
	return b.currentBranch().History
}

func (b *Branching) AddUser(content string) {
	br := b.currentBranch()
	br.History = append(br.History, Message{Role: "user", Content: content})
}

func (b *Branching) AddAssistant(content string) {
	br := b.currentBranch()
	br.History = append(br.History, Message{Role: "assistant", Content: content})
}

func (b *Branching) PostProcess(llmCall func(string, []Message, int) (string, error)) {
	// Branching не использует LLM для пост-обработки
}

func (b *Branching) Reset() {
	b.branches = map[string]*Branch{
		"main": {Name: "main"},
	}
	b.current = "main"
}

func (b *Branching) Info() string {
	result := fmt.Sprintf("Текущая ветка: %s | Всего веток: %d\n", b.current, len(b.branches))
	for name, br := range b.branches {
		marker := "  "
		if name == b.current {
			marker = "> "
		}
		result += fmt.Sprintf("  %s%s (%d сообщений)\n", marker, name, len(br.History))
	}
	return result
}

func (b *Branching) HistoryLen() int {
	return len(b.currentBranch().History)
}

// CreateBranch создаёт новую ветку от текущей точки.
// Копирует текущую историю в новую ветку.
func (b *Branching) CreateBranch(name string) error {
	if _, exists := b.branches[name]; exists {
		return fmt.Errorf("ветка '%s' уже существует", name)
	}

	current := b.currentBranch()
	history := make([]Message, len(current.History))
	copy(history, current.History)

	b.branches[name] = &Branch{
		Name:    name,
		History: history,
	}
	return nil
}

// SwitchBranch переключает на указанную ветку.
func (b *Branching) SwitchBranch(name string) error {
	if _, exists := b.branches[name]; !exists {
		return fmt.Errorf("ветка '%s' не найдена", name)
	}
	b.current = name
	return nil
}

// ListBranches возвращает список имён веток.
func (b *Branching) ListBranches() []string {
	names := make([]string, 0, len(b.branches))
	for name := range b.branches {
		names = append(names, name)
	}
	return names
}

// CurrentBranchName возвращает имя текущей ветки.
func (b *Branching) CurrentBranchName() string {
	return b.current
}

func (b *Branching) currentBranch() *Branch {
	return b.branches[b.current]
}
