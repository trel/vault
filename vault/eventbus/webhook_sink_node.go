package eventbus

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/hashicorp/eventlogger"
	"github.com/hashicorp/go-hclog"
)

// webhookSinkNode is an eventlogger.Node implementation with type "Sink"
var _ eventlogger.Node = (*webhookSinkNode)(nil)

type webhookSinkNode struct {
	// TODO: add bounded deque buffer of *EventReceived
	ctx    context.Context
	logger hclog.Logger
	client *http.Client
	url    string
	format string

	// used to close the connection
	closeOnce  sync.Once
	cancelFunc context.CancelFunc
	pipelineID eventlogger.PipelineID
	broker     *eventlogger.Broker
}

func newWebhookSink(ctx context.Context, logger hclog.Logger, client *http.Client, url, format string) *webhookSinkNode {
	if format == "cloudevents" {
		format += "-json"
	}
	return &webhookSinkNode{
		ctx:    ctx,
		logger: logger,
		client: client,
		url:    url,
		format: format,
	}
}

// Close tells the bus to stop sending us events.
func (node *webhookSinkNode) Close() {
	node.closeOnce.Do(func() {
		defer node.cancelFunc()
		if node.broker != nil {
			err := node.broker.RemovePipeline(eventTypeAll, node.pipelineID)
			if err != nil {
				node.logger.Warn("Error removing pipeline for closing node", "error", err)
			}
		}
		// addSubscriptions(-1)
	})
}

func (node *webhookSinkNode) Process(ctx context.Context, e *eventlogger.Event) (*eventlogger.Event, error) {
	// sends to the webhook URL async in another goroutine
	go func() {
		formattedEvent, ok := e.Format(node.format)
		if !ok {
			node.logger.Error("Could not get event in specified format", "event", e, "format", node.format)
			return
		}
		req, err := http.NewRequest(http.MethodPost, node.url, bytes.NewReader(formattedEvent))
		if err != nil {
			node.logger.Error("Error creating request to send event", "event", e, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := node.client.Do(req)
		if err != nil {
			node.logger.Error("Error sending event", "event", e, "error", err)
			return
		}
		// resp should always be non-nil if err is non-nil.
		if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
			respText, _ := httputil.DumpResponse(resp, true)
			node.logger.Warn("Webhook target did not respond with success status code", "event", e, "code", resp.StatusCode, "resp", respText)
			return
		}
	}()
	return e, nil
}

func (node *webhookSinkNode) Reopen() error {
	return nil
}

func (node *webhookSinkNode) Type() eventlogger.NodeType {
	return eventlogger.NodeTypeSink
}
