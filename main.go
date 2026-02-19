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
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
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

	// Один и тот же промпт — креативная задача, чтобы разница была заметнее
	prompt := "Придумай короткую метафору (2-3 предложения): что такое программирование?"

	temperatures := []float64{0, 0.7, 1.0}

	labels := map[float64]string{
		0:   "ТОЧНЫЙ (temperature = 0)",
		0.7: "СБАЛАНСИРОВАННЫЙ (temperature = 0.7)",
		1.0: "КРЕАТИВНЫЙ (temperature = 1.0)",
	}

	// Для каждой температуры делаем 3 запроса, чтобы оценить разнообразие
	for _, temp := range temperatures {
		fmt.Println("==================================================")
		fmt.Printf("  %s\n", labels[temp])
		fmt.Println("==================================================")

		for i := 1; i <= 3; i++ {
			fmt.Printf("\n--- Попытка %d ---\n", i)
			resp := sendRequest(request{
				Model:       "claude-haiku-4-5-20251001",
				MaxTokens:   200,
				Temperature: temp,
				Messages:    []message{{Role: "user", Content: prompt}},
			})
			printResponse(resp)
		}
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
