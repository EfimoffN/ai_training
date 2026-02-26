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

	a := agent.New(
		apiKey,
		"claude-haiku-4-5-20251001",
		"Ты — полезный ассистент. Отвечай кратко и по делу на русском языке.",
		"history.json",
	)

	if a.HistoryLen() > 0 {
		fmt.Printf("[Загружена история: %d сообщений]\n", a.HistoryLen())
	}

	fmt.Println("Чат с агентом")
	fmt.Println("Команды: 'выход', 'сброс', 'стат'")
	fmt.Println(strings.Repeat("=", 60))

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

		switch strings.ToLower(input) {
		case "выход":
			printFinalStats(a.GetStats())
			return
		case "сброс":
			a.Reset()
			fmt.Println("[История и статистика очищены]")
			continue
		case "стат":
			printDetailedStats(a.GetStats(), a.HistoryLen())
			continue
		}

		reply, err := a.Ask(input)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			continue
		}

		s := a.GetStats()

		fmt.Printf("\nАгент: %s\n", reply)
		fmt.Printf("[токены: %d in / %d out | стоимость запроса: $%.6f | всего: $%.6f]\n",
			s.LastInputTokens, s.LastOutputTokens, s.LastCost, s.TotalCost)
	}
}

func printDetailedStats(s agent.Stats, historyLen int) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("  СТАТИСТИКА СЕССИИ")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Запросов:          %d\n", s.Requests)
	fmt.Printf("  Сообщений:         %d\n", historyLen)
	fmt.Printf("  Input токенов:     %d\n", s.TotalInputTokens)
	fmt.Printf("  Output токенов:    %d\n", s.TotalOutputTokens)
	fmt.Printf("  Всего токенов:     %d\n", s.TotalInputTokens+s.TotalOutputTokens)
	fmt.Printf("  Общая стоимость:   $%.6f\n", s.TotalCost)
	if s.Requests > 0 {
		fmt.Printf("  Среднее input/запрос: %d\n", s.TotalInputTokens/s.Requests)
	}
	fmt.Println(strings.Repeat("-", 40))
}

func printFinalStats(s agent.Stats) {
	if s.Requests == 0 {
		fmt.Println("До свидания!")
		return
	}
	fmt.Println("\n  ИТОГО ЗА СЕССИЮ")
	fmt.Printf("  %d запросов | %d токенов | $%.6f\n",
		s.Requests, s.TotalInputTokens+s.TotalOutputTokens, s.TotalCost)
	fmt.Println("До свидания!")
}
