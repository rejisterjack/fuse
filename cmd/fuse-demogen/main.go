package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// fuse-demogen runs a mock OpenAI upstream and optionally hammers a Fuse proxy
// so the dashboard fills without a real API key.
func main() {
	mockAddr := flag.String("mock-addr", ":18080", "mock OpenAI listen address")
	fuseURL := flag.String("fuse-url", "", "if set, POST fake chat completions through Fuse")
	agent := flag.String("agent", "demo", "X-Fuse-Agent value")
	interval := flag.Duration("interval", 2*time.Second, "traffic interval")
	flag.Parse()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "chatcmpl-demo",
				"object":  "chat.completion",
				"model":   "gpt-4o-mini",
				"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": "ok"}}},
				"usage":   map[string]int{"prompt_tokens": 120, "completion_tokens": 40, "total_tokens": 160},
			})
		})
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true}`))
		})
		log.Printf("mock openai on %s", *mockAddr)
		log.Fatal(http.ListenAndServe(*mockAddr, mux))
	}()

	if *fuseURL == "" {
		select {}
	}

	log.Printf("waiting for fuse at %s", *fuseURL)
	for {
		resp, err := http.Get(*fuseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		time.Sleep(time.Second)
	}
	log.Printf("sending demo traffic to %s", *fuseURL)

	client := &http.Client{Timeout: 10 * time.Second}
	n := 0
	for {
		n++
		body, _ := json.Marshal(map[string]any{
			"model":    "gpt-4o-mini",
			"messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("ping %d", n)}},
		})
		req, err := http.NewRequest(http.MethodPost, *fuseURL+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			log.Printf("req: %v", err)
			time.Sleep(*interval)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fuse-Agent", *agent)
		req.Header.Set("Authorization", "Bearer sk-demo")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("post: %v", err)
			time.Sleep(*interval)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		log.Printf("fuse status=%d body=%s", resp.StatusCode, truncate(string(b), 120))
		time.Sleep(*interval)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
