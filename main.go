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
	"time"
)

const apiURL = "https://api.anthropic.com/v1/messages"

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Цены за 1M токенов (USD)
type pricing struct {
	InputPer1M  float64
	OutputPer1M float64
}

var apiKey string

var prices = map[string]pricing{
	"claude-haiku-4-5-20251001":  {InputPer1M: 0.80, OutputPer1M: 4.00},
	"claude-sonnet-4-5-20250929": {InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-opus-4-6":            {InputPer1M: 15.00, OutputPer1M: 75.00},
}

type model struct {
	ID    string
	Label string
}

func main() {
	apiKey = os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	prompt := "В комнате 3 выключателя. За стеной — 3 лампочки. " +
		"Зайти в комнату с лампочками можно только один раз. " +
		"Как определить, какой выключатель к какой лампочке относится?"

	models := []model{
		{ID: "claude-haiku-4-5-20251001", Label: "СЛАБАЯ — Haiku 3.5 (быстрая, дешёвая)"},
		{ID: "claude-sonnet-4-5-20250929", Label: "СРЕДНЯЯ — Sonnet 4.5 (баланс)"},
		{ID: "claude-opus-4-6", Label: "СИЛЬНАЯ — Opus 4.6 (максимальное качество)"},
	}

	for _, m := range models {
		fmt.Println("==================================================")
		fmt.Printf("  %s\n", m.Label)
		fmt.Println("==================================================")

		start := time.Now()
		resp := sendRequest(request{
			Model:     m.ID,
			MaxTokens: 1024,
			Messages:  []message{{Role: "user", Content: prompt}},
		})
		elapsed := time.Since(start)

		// Ответ
		printResponse(resp)

		// Метрики
		in := resp.Usage.InputTokens
		out := resp.Usage.OutputTokens
		p := prices[m.ID]
		cost := float64(in)/1_000_000*p.InputPer1M + float64(out)/1_000_000*p.OutputPer1M

		fmt.Println("\n--- Метрики ---")
		fmt.Printf("Время ответа:    %s\n", elapsed.Round(time.Millisecond))
		fmt.Printf("Токены (in/out): %d / %d\n", in, out)
		fmt.Printf("Стоимость:       $%.6f\n", cost)
		fmt.Println()
	}
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
