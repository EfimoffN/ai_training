package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const apiURL = "https://api.anthropic.com/v1/messages"

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
	System    string    `json:"system,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

var apiKey string

func main() {
	apiKey = os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	task := "Кирпич стоит 500 рублей и ещё пол кирпича. Сколько стоит кирпич?"

	// === Способ 1: прямой ответ ===
	fmt.Println("==================================================")
	fmt.Println("СПОСОБ 1: Прямой ответ")
	fmt.Println("==================================================")
	resp1 := sendRequest(request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 512,
		Messages:  []message{{Role: "user", Content: task}},
	})
	printResponse(resp1)

	// === Способ 2: пошаговое рассуждение ===
	fmt.Println("\n==================================================")
	fmt.Println("СПОСОБ 2: Пошаговое рассуждение")
	fmt.Println("==================================================")
	resp2 := sendRequest(request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages: []message{{
			Role:    "user",
			Content: task + "\n\nРешай пошагово. На каждом шаге объясни логику.",
		}},
	})
	printResponse(resp2)

	// === Способ 3: сначала составить промпт, потом решить ===
	fmt.Println("\n==================================================")
	fmt.Println("СПОСОБ 3: Модель сама пишет промпт, затем решает")
	fmt.Println("==================================================")

	// Шаг 3а: просим модель составить идеальный промпт
	fmt.Println("--- Шаг А: генерация промпта ---")
	metaResp := sendRequest(request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 512,
		System: "Ты — эксперт по промпт-инженерии. " +
			"Составь идеальный промпт для решения задачи. " +
			"Верни ТОЛЬКО текст промпта, без пояснений.",
		Messages: []message{{Role: "user", Content: task}},
	})
	generatedPrompt := extractText(metaResp)
	fmt.Println(generatedPrompt)

	// Шаг 3б: решаем задачу сгенерированным промптом
	fmt.Println("\n--- Шаг Б: решение по сгенерированному промпту ---")
	resp3 := sendRequest(request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages:  []message{{Role: "user", Content: generatedPrompt}},
	})
	printResponse(resp3)

	// === Способ 4: группа экспертов ===
	fmt.Println("\n==================================================")
	fmt.Println("СПОСОБ 4: Группа экспертов")
	fmt.Println("==================================================")
	resp4 := sendRequest(request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1500,
		System: "В обсуждении участвуют три эксперта:\n\n" +
			"АНАЛИТИК — формализует условие, выделяет ключевые данные и ограничения.\n" +
			"ИНЖЕНЕР — предлагает конкретное решение и проверяет его на примере.\n" +
			"КРИТИК — ищет ошибки в решении и проверяет граничные случаи.\n\n" +
			"Каждый эксперт высказывается по очереди. " +
			"В конце дайте общий согласованный ответ.",
		Messages: []message{{Role: "user", Content: task}},
	})
	printResponse(resp4)
}

func sendRequest(reqBody request) response {
	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("API error %d: %s", resp.StatusCode, respBody)
	}

	var result response
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Fatal(err)
	}

	return result
}

func extractText(resp response) string {
	var parts []string
	for _, block := range resp.Content {
		parts = append(parts, block.Text)
	}
	return strings.Join(parts, "")
}

func printResponse(resp response) {
	fmt.Println(extractText(resp))
}
