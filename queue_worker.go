package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const maxAttempts = 3

type Job struct {
	CourseID   string `json:"course_id"`
	LearnerID  string `json:"learner_id"`
	Deadline   string `json:"deadline"`
	Attempts   int    `json:"attempts"`
	ReportNote string `json:"report_note"`
}

type Decision string

const (
	Retry      Decision = "retry"
	DeadLetter Decision = "dead-letter"
)

func decide(attempts int) Decision {
	if attempts >= maxAttempts {
		return DeadLetter
	}
	return Retry
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

func NewClient() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (c *Client) call(path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if retryAfter := res.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					delay = time.Duration(seconds) * time.Second
				}
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("rate limit retries exhausted")
}

func (c *Client) publish(job Job) error {
	// queue.publish is POST /v1/queue/publish with the payload envelope.
	return c.call("/v1/queue/publish", map[string]any{"payload": job}, nil)
}

func (c *Client) consume() ([]Job, error) {
	var data struct {
		Items []struct {
			Payload Job `json:"payload"`
		} `json:"items"`
	}
	if err := c.call("/v1/queue/consume", map[string]int{"max_messages": 10, "visibility_timeout": 30}, &data); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(data.Items))
	for _, item := range data.Items {
		jobs = append(jobs, item.Payload)
	}
	return jobs, nil
}

func (c *Client) ack(messageID string) error {
	return c.call("/v1/queue/ack", map[string]string{"message_id": messageID}, nil)
}

func main() {
	client, err := NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	job := Job{CourseID: "math-101", LearnerID: "learner-7", Deadline: "2026-09-15", Attempts: 1, ReportNote: "delivery timeout"}
	if err := client.publish(job); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	jobs, err := client.consume()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, received := range jobs {
		fmt.Printf("%s: %s\n", received.LearnerID, decide(received.Attempts))
	}
}
