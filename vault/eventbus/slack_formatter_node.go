package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/eventlogger"
)

// JSON payload with placeholder for a single "block" of markdown text from their Block Kit API.
const slackBlock = `{"blocks":[{"type":"section","text":{"type":"mrkdwn","text":%s}}]}`

type slackFormatterNode struct {
}

var _ eventlogger.Node = &slackFormatterNode{}

func newSlackFormatterNode() *slackFormatterNode {
	return &slackFormatterNode{}
}

// Process formats the data so it can be sent as the body data for a Slack webhook.
// See https://api.slack.com/messaging/webhooks.
func (f *slackFormatterNode) Process(ctx context.Context, e *eventlogger.Event) (*eventlogger.Event, error) {
	payload := map[string]any{
		"blocks": []map[string]any{
			{
				"type": "section",
				"text": map[string]any{
					"type": "mrkdwn",
					"text": "",
				},
			},
		},
	}
	// event := e.Payload.(*logical.EventReceived)
	data, ok := e.Format("cloudevents-json")
	if !ok {
		return nil, fmt.Errorf("failed to get cloudevents-json data as contents for slack webhook")
	}
	dataStr, err := json.Marshal(string(data))
	if err != nil {
		return nil, err
	}
	e.FormattedAs("slack", []byte(fmt.Sprintf(slackBlock, dataStr)))
	return e, nil
}

// Reopen is a no op
func (f *slackFormatterNode) Reopen() error {
	return nil
}

// Type describes the type of the node as a Formatter.
func (f *slackFormatterNode) Type() eventlogger.NodeType {
	return eventlogger.NodeTypeFormatter
}
