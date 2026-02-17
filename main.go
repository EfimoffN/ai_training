package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const apiURL = "https://api.anthropic.com/v1/messages"

type request struct {
	Model         string    `json:"model"`
	MaxTokens     int       `json:"max_tokens"`
	Messages      []message `json:"messages"`
	System        string    `json:"system,omitempty"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	prompt := "Расскажи о языке программирования Go."

	// === Запрос 1: без ограничений ===
	fmt.Println("=== БЕЗ ОГРАНИЧЕНИЙ ===")
	resp1 := sendRequest(apiKey, request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 256,
		Messages:  []message{{Role: "user", Content: prompt}},
	})
	printResponse(resp1)

	// === Запрос 2: с ограничениями ===
	fmt.Println("\n=== С ОГРАНИЧЕНИЯМИ ===")
	resp2 := sendRequest(apiKey, request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 60, // жёсткий лимит токенов
		System: "Отвечай строго в формате:\n" +
			"ЯЗЫК: <название>\n" +
			"ПЛЮСЫ: <3 пункта через запятую>\n" +
			"МИНУСЫ: <3 пункта через запятую>\n" +
			"END",
		StopSequences: []string{"END"}, // остановка при встрече "END"
		Messages:      []message{{Role: "user", Content: prompt}},
	})
	printResponse(resp2)
}

func sendRequest(apiKey string, reqBody request) response {
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

func printResponse(resp response) {
	for _, block := range resp.Content {
		fmt.Println(block.Text)
	}
	fmt.Printf("\n[stop_reason: %s]\n", resp.StopReason)
}
