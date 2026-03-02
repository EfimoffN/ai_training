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
	}

	// По умолчанию — Sliding Window
	a := agent.New(apiKey, model, system, strategies["1"])

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
				fmt.Println("[Неизвестная стратегия. Доступные: 1, 2, 3]")
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
