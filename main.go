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

	// Создаём агента — история автоматически загрузится из файла
	a := agent.New(
		apiKey,
		"claude-haiku-4-5-20251001",
		"Ты — полезный ассистент. Отвечай кратко и по делу на русском языке.",
		"history.json",
	)

	if a.HistoryLen() > 0 {
		fmt.Printf("[Загружена история: %d сообщений]\n", a.HistoryLen())
	}

	fmt.Println("Чат с агентом (введите 'выход' для завершения, 'сброс' для новой темы)")
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
			fmt.Println("До свидания!")
			return
		case "сброс":
			a.Reset()
			fmt.Println("[История очищена — начинаем новый диалог]")
			continue
		}

		reply, err := a.Ask(input)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			continue
		}

		fmt.Printf("\nАгент: %s\n", reply)
		fmt.Printf("[сообщений в истории: %d]\n", a.HistoryLen())
	}
}
