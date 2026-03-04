package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ============================================================
// Модель памяти с тремя слоями
// ============================================================
//
// 1. Краткосрочная (short-term) — последние N сообщений текущего диалога.
//    Скользящее окно, живёт только в рамках сессии.
//
// 2. Рабочая (working) — контекст текущей задачи: цель, ограничения,
//    промежуточные результаты. Очищается при смене задачи.
//
// 3. Долговременная (long-term) — профиль пользователя, принятые решения,
//    накопленные знания. Сохраняется на диск в JSON, переживает сессии.
// ============================================================

// WorkingMemory — рабочая память (данные текущей задачи).
type WorkingMemory struct {
	Task        string   `json:"task"`        // текущая задача
	Goals       []string `json:"goals"`       // цели задачи
	Constraints []string `json:"constraints"` // ограничения
	Progress    []string `json:"progress"`    // промежуточные результаты
}

// LongTermMemory — долговременная память (профиль, решения, знания).
type LongTermMemory struct {
	Profile   map[string]string `json:"profile"`   // имя, язык, предпочтения
	Decisions []string          `json:"decisions"`  // принятые решения
	Knowledge []string          `json:"knowledge"`  // накопленные знания
	UpdatedAt string            `json:"updated_at"` // время обновления
}

// MemoryLayers — стратегия с тремя явными слоями памяти.
type MemoryLayers struct {
	maxShortTerm int
	shortTerm    []Message // краткосрочная: текущий диалог
	fullHistory  []Message // полная история для подсчёта

	working  WorkingMemory  // рабочая: текущая задача
	longTerm LongTermMemory // долговременная: профиль и знания

	profile     UserProfile // персонализация пользователя
	profilePath string      // путь к файлу профиля

	storagePath string // путь к файлу долговременной памяти
	lastSaved   string // контрольная сумма для предотвращения лишних сохранений
}

// NewMemoryLayers создаёт стратегию памяти.
// maxShortTerm — сколько последних сообщений хранить в краткосрочной памяти.
// storagePath — путь к JSON-файлу для долговременной памяти (пустая строка = без персистенции).
// profilePath — путь к JSON-файлу профиля пользователя.
func NewMemoryLayers(maxShortTerm int, storagePath, profilePath string) *MemoryLayers {
	m := &MemoryLayers{
		maxShortTerm: maxShortTerm,
		storagePath:  storagePath,
		profilePath:  profilePath,
		longTerm: LongTermMemory{
			Profile: make(map[string]string),
		},
	}
	m.loadLongTerm()
	m.loadProfile()
	return m
}

func (m *MemoryLayers) Name() string { return "Memory Layers" }

// BuildSystem формирует системный промпт, включая профиль, рабочую и долговременную память.
func (m *MemoryLayers) BuildSystem(base string) string {
	var sb strings.Builder
	sb.WriteString(base)

	// Персонализация — профиль пользователя (самый приоритетный блок)
	if block := m.profile.BuildPromptBlock(); block != "" {
		sb.WriteString(block)
	}

	// Долговременная память — извлечённые факты о пользователе
	if len(m.longTerm.Profile) > 0 {
		sb.WriteString("\n\n[ДОЛГОВРЕМЕННАЯ ПАМЯТЬ — ИЗВЛЕЧЁННЫЕ ФАКТЫ]\n")
		for k, v := range m.longTerm.Profile {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
	}

	// Долговременная память — решения и знания
	if len(m.longTerm.Decisions) > 0 {
		sb.WriteString("\n[ДОЛГОВРЕМЕННАЯ ПАМЯТЬ — РЕШЕНИЯ]\n")
		for _, d := range m.longTerm.Decisions {
			sb.WriteString(fmt.Sprintf("- %s\n", d))
		}
	}
	if len(m.longTerm.Knowledge) > 0 {
		sb.WriteString("\n[ДОЛГОВРЕМЕННАЯ ПАМЯТЬ — ЗНАНИЯ]\n")
		for _, k := range m.longTerm.Knowledge {
			sb.WriteString(fmt.Sprintf("- %s\n", k))
		}
	}

	// Рабочая память — текущая задача
	if m.working.Task != "" {
		sb.WriteString("\n[РАБОЧАЯ ПАМЯТЬ — ТЕКУЩАЯ ЗАДАЧА]\n")
		sb.WriteString(fmt.Sprintf("Задача: %s\n", m.working.Task))
		if len(m.working.Goals) > 0 {
			sb.WriteString("Цели:\n")
			for _, g := range m.working.Goals {
				sb.WriteString(fmt.Sprintf("  - %s\n", g))
			}
		}
		if len(m.working.Constraints) > 0 {
			sb.WriteString("Ограничения:\n")
			for _, c := range m.working.Constraints {
				sb.WriteString(fmt.Sprintf("  - %s\n", c))
			}
		}
		if len(m.working.Progress) > 0 {
			sb.WriteString("Прогресс:\n")
			for _, p := range m.working.Progress {
				sb.WriteString(fmt.Sprintf("  - %s\n", p))
			}
		}
	}

	return sb.String()
}

// BuildMessages возвращает только краткосрочную память (последние N сообщений).
func (m *MemoryLayers) BuildMessages() []Message {
	if len(m.shortTerm) <= m.maxShortTerm {
		return m.shortTerm
	}
	return m.shortTerm[len(m.shortTerm)-m.maxShortTerm:]
}

func (m *MemoryLayers) AddUser(content string) {
	m.shortTerm = append(m.shortTerm, Message{Role: "user", Content: content})
	m.fullHistory = append(m.fullHistory, Message{Role: "user", Content: content})
	m.trimShortTerm()
}

func (m *MemoryLayers) AddAssistant(content string) {
	m.shortTerm = append(m.shortTerm, Message{Role: "assistant", Content: content})
	m.fullHistory = append(m.fullHistory, Message{Role: "assistant", Content: content})
	m.trimShortTerm()
}

// PostProcess — после каждого обмена LLM классифицирует новую информацию
// и распределяет по слоям памяти.
func (m *MemoryLayers) PostProcess(llmCall func(string, []Message, int) (string, error)) {
	if len(m.fullHistory) < 2 {
		return
	}

	// Берём последнюю пару user+assistant
	recent := m.fullHistory[len(m.fullHistory)-2:]

	var sb strings.Builder
	sb.WriteString("Последний обмен:\n")
	for _, msg := range recent {
		if msg.Role == "user" {
			sb.WriteString("Пользователь: ")
		} else {
			sb.WriteString("Ассистент: ")
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}

	// Текущее состояние памяти для контекста
	sb.WriteString("\n--- Текущая рабочая память ---\n")
	if m.working.Task != "" {
		sb.WriteString(fmt.Sprintf("Задача: %s\n", m.working.Task))
	} else {
		sb.WriteString("Задача: (не определена)\n")
	}

	sb.WriteString("\n--- Текущая долговременная память ---\n")
	if len(m.longTerm.Profile) > 0 {
		sb.WriteString("Профиль: ")
		for k, v := range m.longTerm.Profile {
			sb.WriteString(fmt.Sprintf("%s=%s; ", k, v))
		}
		sb.WriteString("\n")
	}

	system := `Ты — менеджер памяти ассистента. Проанализируй последний обмен и определи, какую информацию нужно сохранить.

Ответь СТРОГО в формате JSON (без markdown, без обёрток):
{
  "working": {
    "task": "описание текущей задачи или пустая строка если не изменилась",
    "goals": ["новые цели, если появились"],
    "constraints": ["новые ограничения, если появились"],
    "progress": ["новые промежуточные результаты, если есть"]
  },
  "long_term": {
    "profile": {"ключ": "значение — новые факты о пользователе"},
    "decisions": ["новые решения, если приняты"],
    "knowledge": ["новые важные знания, если появились"]
  },
  "reasoning": "краткое пояснение, почему именно эти данные и в эти слои"
}

Правила:
- Если новой информации для слоя нет — оставь пустые массивы/объекты
- В working помещай только то, что относится к ТЕКУЩЕЙ задаче
- В long_term помещай стабильные факты, которые пригодятся в будущих диалогах
- Не дублируй то, что уже есть в памяти`

	result, err := llmCall(system, []Message{{Role: "user", Content: sb.String()}}, 500)
	if err != nil {
		return
	}

	m.applyMemoryUpdate(result)
}

// memoryUpdate — структура ответа от LLM-менеджера памяти.
type memoryUpdate struct {
	Working struct {
		Task        string   `json:"task"`
		Goals       []string `json:"goals"`
		Constraints []string `json:"constraints"`
		Progress    []string `json:"progress"`
	} `json:"working"`
	LongTerm struct {
		Profile   map[string]string `json:"profile"`
		Decisions []string          `json:"decisions"`
		Knowledge []string          `json:"knowledge"`
	} `json:"long_term"`
	Reasoning string `json:"reasoning"`
}

func (m *MemoryLayers) applyMemoryUpdate(raw string) {
	// Пробуем извлечь JSON из ответа (LLM иногда оборачивает в markdown)
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "}"); idx >= 0 {
		raw = raw[:idx+1]
	}

	var update memoryUpdate
	if err := json.Unmarshal([]byte(raw), &update); err != nil {
		return
	}

	// Обновляем рабочую память
	if update.Working.Task != "" {
		m.working.Task = update.Working.Task
	}
	m.working.Goals = appendUnique(m.working.Goals, update.Working.Goals)
	m.working.Constraints = appendUnique(m.working.Constraints, update.Working.Constraints)
	m.working.Progress = append(m.working.Progress, update.Working.Progress...)

	// Обновляем долговременную память
	if m.longTerm.Profile == nil {
		m.longTerm.Profile = make(map[string]string)
	}
	for k, v := range update.LongTerm.Profile {
		if v != "" {
			m.longTerm.Profile[k] = v
		}
	}
	m.longTerm.Decisions = appendUnique(m.longTerm.Decisions, update.LongTerm.Decisions)
	m.longTerm.Knowledge = appendUnique(m.longTerm.Knowledge, update.LongTerm.Knowledge)
	m.longTerm.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

	m.saveLongTerm()
}

func (m *MemoryLayers) Reset() {
	m.shortTerm = nil
	m.fullHistory = nil
	m.working = WorkingMemory{}
	// Долговременная память НЕ сбрасывается — она персистентна
}

// ResetWorking очищает только рабочую память (смена задачи).
func (m *MemoryLayers) ResetWorking() {
	m.working = WorkingMemory{}
}

// ResetLongTerm полностью очищает долговременную память.
func (m *MemoryLayers) ResetLongTerm() {
	m.longTerm = LongTermMemory{Profile: make(map[string]string)}
	m.saveLongTerm()
}

func (m *MemoryLayers) Info() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Краткосрочная: %d/%d сообщений\n", len(m.shortTerm), m.maxShortTerm))

	sb.WriteString("  Рабочая: ")
	if m.working.Task != "" {
		sb.WriteString(fmt.Sprintf("задача=%q, %d целей, %d шагов\n",
			m.working.Task, len(m.working.Goals), len(m.working.Progress)))
	} else {
		sb.WriteString("(пусто)\n")
	}

	sb.WriteString(fmt.Sprintf("  Долговременная: %d в профиле, %d решений, %d знаний",
		len(m.longTerm.Profile), len(m.longTerm.Decisions), len(m.longTerm.Knowledge)))
	if m.longTerm.UpdatedAt != "" {
		sb.WriteString(fmt.Sprintf(" (обновлено: %s)", m.longTerm.UpdatedAt))
	}
	return sb.String()
}

func (m *MemoryLayers) HistoryLen() int {
	return len(m.shortTerm)
}

// --- Управление профилем ---

// GetProfile возвращает текущий профиль.
func (m *MemoryLayers) GetProfile() UserProfile {
	return m.profile
}

// SetProfile устанавливает профиль и сохраняет на диск.
func (m *MemoryLayers) SetProfile(p UserProfile) {
	m.profile = p
	m.saveProfile()
}

// ResetProfile сбрасывает профиль.
func (m *MemoryLayers) ResetProfile() {
	m.profile = UserProfile{}
	m.saveProfile()
}

func (m *MemoryLayers) saveProfile() {
	if m.profilePath == "" {
		return
	}
	_ = SaveProfile(m.profilePath, &m.profile)
}

func (m *MemoryLayers) loadProfile() {
	if m.profilePath == "" {
		return
	}
	if p, err := LoadProfile(m.profilePath); err == nil {
		m.profile = *p
	}
}

// --- Доступ к слоям для отображения в main ---

// GetShortTerm возвращает краткосрочную память.
func (m *MemoryLayers) GetShortTerm() []Message {
	return m.shortTerm
}

// GetWorking возвращает рабочую память.
func (m *MemoryLayers) GetWorking() WorkingMemory {
	return m.working
}

// GetLongTerm возвращает долговременную память.
func (m *MemoryLayers) GetLongTerm() LongTermMemory {
	return m.longTerm
}

// --- Персистенция долговременной памяти ---

func (m *MemoryLayers) saveLongTerm() {
	if m.storagePath == "" {
		return
	}
	data, err := json.MarshalIndent(m.longTerm, "", "  ")
	if err != nil {
		return
	}
	// Не перезаписываем, если ничего не изменилось
	checksum := string(data)
	if checksum == m.lastSaved {
		return
	}
	m.lastSaved = checksum
	_ = os.WriteFile(m.storagePath, data, 0644)
}

func (m *MemoryLayers) loadLongTerm() {
	if m.storagePath == "" {
		return
	}
	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return
	}
	var lt LongTermMemory
	if err := json.Unmarshal(data, &lt); err != nil {
		return
	}
	if lt.Profile == nil {
		lt.Profile = make(map[string]string)
	}
	m.longTerm = lt
	m.lastSaved = string(data)
}

func (m *MemoryLayers) trimShortTerm() {
	if len(m.shortTerm) > m.maxShortTerm {
		excess := len(m.shortTerm) - m.maxShortTerm
		m.shortTerm = m.shortTerm[excess:]
	}
}

// appendUnique добавляет элементы, которых ещё нет в списке.
func appendUnique(existing, new []string) []string {
	set := make(map[string]bool, len(existing))
	for _, s := range existing {
		set[s] = true
	}
	for _, s := range new {
		if s != "" && !set[s] {
			existing = append(existing, s)
			set[s] = true
		}
	}
	return existing
}
