package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// UserProfile — структурированный профиль пользователя.
// Определяет персонализацию: как ассистент общается, в каком формате отвечает,
// на каком уровне объясняет, какие ограничения соблюдает.
type UserProfile struct {
	Name        string   `json:"name"`        // имя пользователя
	Style       string   `json:"style"`       // стиль: formal / informal / technical
	Format      string   `json:"format"`      // формат: brief / detailed / structured
	Language    string   `json:"language"`     // язык общения
	Expertise   string   `json:"expertise"`   // уровень: beginner / intermediate / expert
	Constraints []string `json:"constraints"` // ограничения и правила
	Interests   []string `json:"interests"`   // области интересов
}

// IsEmpty проверяет, пуст ли профиль.
func (p *UserProfile) IsEmpty() bool {
	return p.Name == "" && p.Style == "" && p.Format == "" &&
		p.Expertise == "" && len(p.Constraints) == 0 && len(p.Interests) == 0
}

// BuildPromptBlock формирует блок для системного промпта.
func (p *UserProfile) BuildPromptBlock() string {
	if p.IsEmpty() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n[ПРОФИЛЬ ПОЛЬЗОВАТЕЛЯ — ПЕРСОНАЛИЗАЦИЯ]\n")
	sb.WriteString("Адаптируй ВСЕ свои ответы под этот профиль:\n")

	if p.Name != "" {
		sb.WriteString(fmt.Sprintf("- Имя: %s\n", p.Name))
	}
	if p.Style != "" {
		sb.WriteString(fmt.Sprintf("- Стиль общения: %s\n", p.styleDescription()))
	}
	if p.Format != "" {
		sb.WriteString(fmt.Sprintf("- Формат ответов: %s\n", p.formatDescription()))
	}
	if p.Language != "" {
		sb.WriteString(fmt.Sprintf("- Язык: %s\n", p.Language))
	}
	if p.Expertise != "" {
		sb.WriteString(fmt.Sprintf("- Уровень пользователя: %s\n", p.expertiseDescription()))
	}
	if len(p.Interests) > 0 {
		sb.WriteString(fmt.Sprintf("- Области интересов: %s\n", strings.Join(p.Interests, ", ")))
	}
	if len(p.Constraints) > 0 {
		sb.WriteString("- ОБЯЗАТЕЛЬНЫЕ ПРАВИЛА:\n")
		for _, c := range p.Constraints {
			sb.WriteString(fmt.Sprintf("  * %s\n", c))
		}
	}

	return sb.String()
}

func (p *UserProfile) styleDescription() string {
	switch p.Style {
	case "formal":
		return "формальный — обращайся на «Вы», используй деловой тон"
	case "informal":
		return "неформальный — обращайся на «ты», используй дружеский тон"
	case "technical":
		return "технический — минимум слов, максимум кода и терминов"
	default:
		return p.Style
	}
}

func (p *UserProfile) formatDescription() string {
	switch p.Format {
	case "brief":
		return "краткий — 1-3 предложения, только суть"
	case "detailed":
		return "подробный — объясняй шаг за шагом, с примерами"
	case "structured":
		return "структурированный — используй списки, заголовки, разделы"
	default:
		return p.Format
	}
}

func (p *UserProfile) expertiseDescription() string {
	switch p.Expertise {
	case "beginner":
		return "новичок — объясняй простым языком, избегай жаргона, давай аналогии"
	case "intermediate":
		return "средний — можно использовать термины, но объясняй сложные концепции"
	case "expert":
		return "эксперт — используй термины свободно, не объясняй базовые вещи"
	default:
		return p.Expertise
	}
}

// Display возвращает читаемое представление профиля.
func (p *UserProfile) Display() string {
	if p.IsEmpty() {
		return "(профиль не задан)"
	}

	var sb strings.Builder
	if p.Name != "" {
		sb.WriteString(fmt.Sprintf("  Имя:       %s\n", p.Name))
	}
	if p.Style != "" {
		sb.WriteString(fmt.Sprintf("  Стиль:     %s (%s)\n", p.Style, p.styleDescription()))
	}
	if p.Format != "" {
		sb.WriteString(fmt.Sprintf("  Формат:    %s (%s)\n", p.Format, p.formatDescription()))
	}
	if p.Language != "" {
		sb.WriteString(fmt.Sprintf("  Язык:      %s\n", p.Language))
	}
	if p.Expertise != "" {
		sb.WriteString(fmt.Sprintf("  Уровень:   %s (%s)\n", p.Expertise, p.expertiseDescription()))
	}
	if len(p.Interests) > 0 {
		sb.WriteString(fmt.Sprintf("  Интересы:  %s\n", strings.Join(p.Interests, ", ")))
	}
	if len(p.Constraints) > 0 {
		sb.WriteString("  Правила:\n")
		for _, c := range p.Constraints {
			sb.WriteString(fmt.Sprintf("    - %s\n", c))
		}
	}
	return sb.String()
}

// --- Пресеты профилей ---

// PresetProfiles возвращает набор готовых профилей для разных типов пользователей.
func PresetProfiles() map[string]UserProfile {
	return map[string]UserProfile{
		"новичок": {
			Name:      "",
			Style:     "informal",
			Format:    "detailed",
			Language:  "русский",
			Expertise: "beginner",
			Constraints: []string{
				"Объясняй каждый шаг",
				"Давай аналогии из реальной жизни",
				"Предлагай что попробовать дальше",
			},
			Interests: []string{"обучение программированию"},
		},
		"разработчик": {
			Name:      "",
			Style:     "technical",
			Format:    "brief",
			Language:  "русский",
			Expertise: "expert",
			Constraints: []string{
				"Код важнее текста",
				"Показывай примеры на Go",
				"Указывай edge-cases",
			},
			Interests: []string{"Go", "системное программирование", "архитектура"},
		},
		"менеджер": {
			Name:      "",
			Style:     "formal",
			Format:    "structured",
			Language:  "русский",
			Expertise: "intermediate",
			Constraints: []string{
				"Используй бизнес-терминологию",
				"Структурируй ответы с заголовками",
				"Выделяй риски и рекомендации",
			},
			Interests: []string{"управление проектами", "аналитика", "процессы"},
		},
	}
}

// --- Персистенция профиля ---

// SaveProfile сохраняет профиль в JSON-файл.
func SaveProfile(path string, p *UserProfile) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadProfile загружает профиль из JSON-файла.
func LoadProfile(path string) (*UserProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &UserProfile{}, err
	}
	var p UserProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return &UserProfile{}, err
	}
	return &p, nil
}
