package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8watch/internal/storage"
)

type SlackNotifier struct {
	webhookURL string
	enabled    bool
	client     *http.Client
}

type slackMessage struct {
	Text        string            `json:"text,omitempty"`
	Blocks      []slackBlock      `json:"blocks,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackBlock struct {
	Type string        `json:"type"`
	Text *slackTextObj `json:"text,omitempty"`
}

type slackTextObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackAttachment struct {
	Color  string       `json:"color,omitempty"`
	Title  string       `json:"title,omitempty"`
	Text   string       `json:"text,omitempty"`
	Fields []slackField `json:"fields,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		enabled:    webhookURL != "",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsEnabled returns whether Slack notifications are enabled
func (s *SlackNotifier) IsEnabled() bool {
	return s.enabled
}

// NotifyChange sends a notification about a resource change
func (s *SlackNotifier) NotifyChange(event *storage.ChangeEvent) error {
	if !s.enabled {
		return nil
	}

	// Only notify on critical changes (MODIFIED and DELETED)
	if event.Action != "MODIFIED" && event.Action != "DELETED" {
		return nil
	}

	color := s.getColorForAction(event.Action)
	emoji := s.getEmojiForKind(event.Kind)

	msg := slackMessage{
		Attachments: []slackAttachment{
			{
				Color: color,
				Title: fmt.Sprintf("%s %s %s in %s", emoji, event.Kind, event.Action, event.Namespace),
				Fields: []slackField{
					{
						Title: "Resource",
						Value: fmt.Sprintf("`%s/%s`", event.Namespace, event.Name),
						Short: true,
					},
					{
						Title: "Action",
						Value: event.Action,
						Short: true,
					},
				},
			},
		},
	}

	// Add change details
	if event.Diff != "" {
		// Truncate diff if too long
		diff := event.Diff
		if len(diff) > 500 {
			diff = diff[:500] + "...\n_(truncated)_"
		}
		msg.Attachments[0].Text = fmt.Sprintf("```\n%s\n```", diff)
	}

	// Add image changes for deployments
	if event.ImageBefore != "" && event.ImageAfter != "" {
		msg.Attachments[0].Fields = append(msg.Attachments[0].Fields, slackField{
			Title: "Image Change",
			Value: fmt.Sprintf("From: `%s`\nTo: `%s`", event.ImageBefore, event.ImageAfter),
			Short: false,
		})
	}

	return s.sendMessage(msg)
}

// sendMessage sends a message to Slack
func (s *SlackNotifier) sendMessage(msg slackMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned non-200 status code: %d", resp.StatusCode)
	}

	return nil
}

// getColorForAction returns Slack color for action
func (s *SlackNotifier) getColorForAction(action string) string {
	switch action {
	case "ADDED":
		return "good" // green
	case "DELETED":
		return "danger" // red
	case "MODIFIED":
		return "warning" // yellow
	default:
		return "#808080" // gray
	}
}

// getEmojiForKind returns emoji for resource kind
func (s *SlackNotifier) getEmojiForKind(kind string) string {
	switch kind {
	case "Deployment":
		return "🚀"
	case "ConfigMap":
		return "📝"
	case "Secret":
		return "🔐"
	case "Service":
		return "🌐"
	case "Ingress":
		return "🚪"
	case "StatefulSet":
		return "💾"
	case "DaemonSet":
		return "👹"
	case "CronJob":
		return "⏰"
	case "Job":
		return "⚙️"
	default:
		return "📦"
	}
}

// TestConnection sends a test message to verify Slack webhook
func (s *SlackNotifier) TestConnection() error {
	if !s.enabled {
		return fmt.Errorf("slack notifier is not enabled")
	}

	msg := slackMessage{
		Text: "🎉 K8Watch notifications are now active!",
		Attachments: []slackAttachment{
			{
				Color: "good",
				Text:  "You will receive notifications for critical Kubernetes resource changes.",
			},
		},
	}

	return s.sendMessage(msg)
}
