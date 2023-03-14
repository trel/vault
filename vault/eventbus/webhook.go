package eventbus

import (
	"context"

	"github.com/hashicorp/eventlogger"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/helper/namespace"
)

func (bus *EventBus) RegisterWebhook(ctx context.Context, ns *namespace.Namespace, url, pattern, format string) error {
	if !bus.started.Load() {
		return ErrNotStarted
	}

	pipelineID, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	filterNodeID, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	filterNode := newFilterNode(ns, pattern)
	err = bus.broker.RegisterNode(eventlogger.NodeID(filterNodeID), filterNode)
	if err != nil {
		return err
	}

	sinkNodeID, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	webhookSink := newWebhookSink(ctx, bus.logger, cleanhttp.DefaultClient(), url, format)
	err = bus.broker.RegisterNode(eventlogger.NodeID(sinkNodeID), webhookSink)
	if err != nil {
		return err
	}

	nodes := []eventlogger.NodeID{eventlogger.NodeID(filterNodeID), bus.formatterNodeID}
	if format == "slack" {
		nodes = append(nodes, bus.slackFormatterNodeID)
	}
	nodes = append(nodes, eventlogger.NodeID(sinkNodeID))

	pipeline := eventlogger.Pipeline{
		PipelineID: eventlogger.PipelineID(pipelineID),
		EventType:  eventTypeAll,
		NodeIDs:    nodes,
	}
	err = bus.broker.RegisterPipeline(pipeline)
	if err != nil {
		return err
	}

	// TODO: Does this count as a subscription for metrics purposes? Probably not.
	// addSubscriptions(1)
	// add info needed to cancel the subscription
	webhookSink.pipelineID = eventlogger.PipelineID(pipelineID)

	// TODO: Now write to durable storage so that it still exists after restart.

	return nil
}
