package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"ai_training/agent"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	model := "claude-haiku-4-5-20251001"
	system := "Ты — полезный ассистент. Отвечай кратко и по делу на русском языке."

	// Создаём все стратегии
	strategies := map[string]agent.Strategy{
		"1": agent.NewSlidingWindow(6),
		"2": agent.NewStickyFacts(6),
		"3": agent.NewBranching(),
		"4": agent.NewMemoryLayers(6, "long_term_memory.json", "user_profile.json"),
	}

	// По умолчанию — Memory Layers
	a := agent.New(apiKey, model, system, strategies["4"])

	printHelp(a)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nВы: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		cmd := strings.ToLower(input)

		// DEBUG: показываем тип стратегии при вводе "дебаг"
		if cmd == "дебаг" {
			fmt.Printf("[DEBUG] strategy type: %T\n", a.GetStrategy())
			continue
		}

		// Общие команды
		switch cmd {
		case "выход":
			printFinalStats(a)
			return
		case "сброс":
			a.Reset()
			fmt.Println("[Очищено]")
			continue
		case "стат":
			printStats(a)
			continue
		case "инфо":
			fmt.Printf("\n[%s] %s\n", a.GetStrategy().Name(), a.GetStrategy().Info())
			continue
		case "помощь":
			printHelp(a)
			continue
		}

		// Переключение стратегии
		if key, ok := strings.CutPrefix(cmd, "стратегия "); ok {
			if s, ok := strategies[key]; ok {
				a.SetStrategy(s)
				fmt.Printf("[Стратегия: %s]\n", s.Name())
			} else {
				fmt.Println("[Неизвестная стратегия. Доступные: 1, 2, 3, 4]")
			}
			continue
		}

		// Команды Branching
		if br, ok := a.GetStrategy().(*agent.Branching); ok {
			if name, ok := strings.CutPrefix(cmd, "ветка "); ok {
				if err := br.CreateBranch(name); err != nil {
					fmt.Printf("[Ошибка: %v]\n", err)
				} else {
					fmt.Printf("[Создана ветка '%s' (копия текущей)]\n", name)
				}
				continue
			}
			if name, ok := strings.CutPrefix(cmd, "перейти "); ok {
				if err := br.SwitchBranch(name); err != nil {
					fmt.Printf("[Ошибка: %v]\n", err)
				} else {
					fmt.Printf("[Переключено на ветку '%s' (%d сообщ.)]\n", name, br.HistoryLen())
				}
				continue
			}
			if cmd == "ветки" {
				fmt.Println()
				fmt.Println(br.Info())
				continue
			}
		}

		// Команда Facts
		if facts, ok := a.GetStrategy().(*agent.StickyFacts); ok {
			if cmd == "факты" {
				if f := facts.GetFacts(); f != "" {
					fmt.Printf("\n[ФАКТЫ]\n%s\n", f)
				} else {
					fmt.Println("[Факты пока не извлечены]")
				}
				continue
			}
		}

		// Команды Memory Layers
		if mem, ok := a.GetStrategy().(*agent.MemoryLayers); ok {
			switch cmd {
			case "память":
				printMemoryLayers(mem)
				continue
			case "кратко":
				printShortTerm(mem)
				continue
			case "рабочая":
				printWorkingMemory(mem)
				continue
			case "долго":
				printLongTerm(mem)
				continue
			case "сброс задачи":
				mem.ResetWorking()
				fmt.Println("[Рабочая память очищена]")
				continue
			case "сброс долго":
				mem.ResetLongTerm()
				fmt.Println("[Долговременная память очищена]")
				continue
			case "профиль":
				printProfile(mem)
				continue
			case "профили":
				printPresets()
				continue
			case "сброс профиля":
				mem.ResetProfile()
				fmt.Println("[Профиль сброшен]")
				continue
			}

			// Применение пресета: "пресет новичок"
			if name, ok := strings.CutPrefix(cmd, "пресет "); ok {
				presets := agent.PresetProfiles()
				if p, exists := presets[name]; exists {
					// Сохраняем имя из текущего профиля
					if cur := mem.GetProfile(); cur.Name != "" {
						p.Name = cur.Name
					}
					mem.SetProfile(p)
					fmt.Printf("[Профиль: %s]\n", name)
					printProfile(mem)
				} else {
					fmt.Println("[Неизвестный пресет. Доступные:]")
					printPresets()
				}
				continue
			}

			// Установка полей: "имя Алексей", "стиль formal", и т.д.
			if val, ok := strings.CutPrefix(cmd, "имя "); ok {
				p := mem.GetProfile()
				p.Name = val
				mem.SetProfile(p)
				fmt.Printf("[Имя: %s]\n", val)
				continue
			}
			if val, ok := strings.CutPrefix(cmd, "стиль "); ok {
				p := mem.GetProfile()
				p.Style = val
				mem.SetProfile(p)
				fmt.Printf("[Стиль: %s]\n", val)
				continue
			}
			if val, ok := strings.CutPrefix(cmd, "формат "); ok {
				p := mem.GetProfile()
				p.Format = val
				mem.SetProfile(p)
				fmt.Printf("[Формат: %s]\n", val)
				continue
			}
			if val, ok := strings.CutPrefix(cmd, "уровень "); ok {
				p := mem.GetProfile()
				p.Expertise = val
				mem.SetProfile(p)
				fmt.Printf("[Уровень: %s]\n", val)
				continue
			}
		}

		// Обычное сообщение — отправляем агенту
		reply, err := a.Ask(input)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			continue
		}

		s := a.GetStats()
		fmt.Printf("\nАгент: %s\n", reply)
		fmt.Printf("[%s | %d in / %d out | $%.6f | история: %d]\n",
			a.GetStrategy().Name(), s.LastInputTokens, s.LastOutputTokens, s.TotalCost, a.HistoryLen())
	}
}

func printHelp(a *agent.Agent) {
	fmt.Println()
	fmt.Printf("Стратегия: %s\n", a.GetStrategy().Name())
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Команды:")
	fmt.Println("  стратегия 1  — Sliding Window (последние N сообщений)")
	fmt.Println("  стратегия 2  — Sticky Facts (факты + последние N)")
	fmt.Println("  стратегия 3  — Branching (ветки диалога)")
	fmt.Println("  стратегия 4  — Memory Layers (3 слоя памяти)")
	fmt.Println()
	fmt.Println("  стат         — статистика токенов")
	fmt.Println("  инфо         — состояние текущей стратегии")
	fmt.Println("  сброс        — очистить историю")
	fmt.Println("  помощь       — эта справка")
	fmt.Println("  выход        — завершить")
	fmt.Println()
	fmt.Println("Sticky Facts:")
	fmt.Println("  факты        — показать извлечённые факты")
	fmt.Println()
	fmt.Println("Branching:")
	fmt.Println("  ветка <имя>  — создать ветку от текущей точки")
	fmt.Println("  перейти <имя>— переключиться на ветку")
	fmt.Println("  ветки        — список всех веток")
	fmt.Println()
	fmt.Println("Memory Layers:")
	fmt.Println("  память       — все три слоя памяти")
	fmt.Println("  кратко       — краткосрочная (текущий диалог)")
	fmt.Println("  рабочая      — рабочая (текущая задача)")
	fmt.Println("  долго        — долговременная (профиль, знания)")
	fmt.Println("  сброс задачи — очистить рабочую память")
	fmt.Println("  сброс долго  — очистить долговременную память")
	fmt.Println()
	fmt.Println("Персонализация:")
	fmt.Println("  профиль        — текущий профиль")
	fmt.Println("  профили        — список пресетов")
	fmt.Println("  пресет <имя>   — применить пресет (новичок/разработчик/менеджер)")
	fmt.Println("  имя <значение> — установить имя")
	fmt.Println("  стиль <знач>   — formal / informal / technical")
	fmt.Println("  формат <знач>  — brief / detailed / structured")
	fmt.Println("  уровень <знач> — beginner / intermediate / expert")
	fmt.Println("  сброс профиля  — очистить профиль")
	fmt.Println(strings.Repeat("=", 60))
}

func printStats(a *agent.Agent) {
	s := a.GetStats()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 45))
	fmt.Printf("  СТАТИСТИКА [%s]\n", a.GetStrategy().Name())
	fmt.Println(strings.Repeat("-", 45))
	fmt.Printf("  Запросов к API:       %d\n", s.Requests)
	fmt.Printf("  Сообщений в истории:  %d\n", a.HistoryLen())
	fmt.Printf("  Input токенов:        %d\n", s.TotalInputTokens)
	fmt.Printf("  Output токенов:       %d\n", s.TotalOutputTokens)
	fmt.Printf("  Всего токенов:        %d\n", s.TotalInputTokens+s.TotalOutputTokens)
	fmt.Printf("  Общая стоимость:      $%.6f\n", s.TotalCost)
	if s.Requests > 0 {
		fmt.Printf("  Среднее input/запрос: %d\n", s.TotalInputTokens/s.Requests)
	}
	fmt.Printf("  Стратегия:            %s\n", a.GetStrategy().Info())
	fmt.Println(strings.Repeat("-", 45))
}

func printFinalStats(a *agent.Agent) {
	s := a.GetStats()
	if s.Requests == 0 {
		fmt.Println("До свидания!")
		return
	}
	fmt.Printf("\n  ИТОГО [%s]: %d запросов | %d токенов | $%.6f\n",
		a.GetStrategy().Name(), s.Requests,
		s.TotalInputTokens+s.TotalOutputTokens, s.TotalCost)
	fmt.Println("До свидания!")
}

// --- Функции вывода слоёв памяти ---

func printMemoryLayers(mem *agent.MemoryLayers) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("  МОДЕЛЬ ПАМЯТИ — ВСЕ СЛОИ")
	fmt.Println(strings.Repeat("=", 50))
	printShortTerm(mem)
	printWorkingMemory(mem)
	printLongTerm(mem)
}

func printShortTerm(mem *agent.MemoryLayers) {
	msgs := mem.GetShortTerm()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  КРАТКОСРОЧНАЯ ПАМЯТЬ (%d сообщений)\n", len(msgs))
	fmt.Println("  Содержит: последние сообщения текущего диалога")
	fmt.Println(strings.Repeat("-", 50))
	if len(msgs) == 0 {
		fmt.Println("  (пусто)")
		return
	}
	for i, m := range msgs {
		role := "User"
		if m.Role == "assistant" {
			role = "Asst"
		}
		text := m.Content
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		fmt.Printf("  %d. [%s] %s\n", i+1, role, text)
	}
}

func printWorkingMemory(mem *agent.MemoryLayers) {
	w := mem.GetWorking()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("  РАБОЧАЯ ПАМЯТЬ (текущая задача)")
	fmt.Println("  Содержит: цель, ограничения, прогресс")
	fmt.Println(strings.Repeat("-", 50))
	if w.Task == "" {
		fmt.Println("  (задача не определена)")
		return
	}
	fmt.Printf("  Задача: %s\n", w.Task)
	if len(w.Goals) > 0 {
		fmt.Println("  Цели:")
		for _, g := range w.Goals {
			fmt.Printf("    - %s\n", g)
		}
	}
	if len(w.Constraints) > 0 {
		fmt.Println("  Ограничения:")
		for _, c := range w.Constraints {
			fmt.Printf("    - %s\n", c)
		}
	}
	if len(w.Progress) > 0 {
		fmt.Println("  Прогресс:")
		for _, p := range w.Progress {
			fmt.Printf("    - %s\n", p)
		}
	}
}

func printLongTerm(mem *agent.MemoryLayers) {
	lt := mem.GetLongTerm()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("  ДОЛГОВРЕМЕННАЯ ПАМЯТЬ (персистентная)")
	fmt.Println("  Содержит: профиль, решения, знания")
	fmt.Println(strings.Repeat("-", 50))
	if len(lt.Profile) == 0 && len(lt.Decisions) == 0 && len(lt.Knowledge) == 0 {
		fmt.Println("  (пусто)")
		return
	}
	if len(lt.Profile) > 0 {
		fmt.Println("  Профиль:")
		for k, v := range lt.Profile {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}
	if len(lt.Decisions) > 0 {
		fmt.Println("  Решения:")
		for _, d := range lt.Decisions {
			fmt.Printf("    - %s\n", d)
		}
	}
	if len(lt.Knowledge) > 0 {
		fmt.Println("  Знания:")
		for _, k := range lt.Knowledge {
			fmt.Printf("    - %s\n", k)
		}
	}
	if lt.UpdatedAt != "" {
		fmt.Printf("  Обновлено: %s\n", lt.UpdatedAt)
	}
}

// --- Функции профиля ---

func printProfile(mem *agent.MemoryLayers) {
	p := mem.GetProfile()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("  ПРОФИЛЬ ПОЛЬЗОВАТЕЛЯ")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Print(p.Display())
}

func printPresets() {
	presets := agent.PresetProfiles()
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("  ДОСТУПНЫЕ ПРЕСЕТЫ")
	fmt.Println(strings.Repeat("-", 50))
	for name, p := range presets {
		fmt.Printf("\n  [%s]\n", name)
		fmt.Print(p.Display())
	}
}
