package vault

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/vault/helper/namespace"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *SystemBackend) eventWebhookPaths() []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "events/webhooks",
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ListOperation: &framework.PathOperation{
					Callback: b.handleListWebhooks,
					Responses: map[int][]framework.Response{
						http.StatusOK: {{
							Description: "OK",
						}},
					},
					Summary: "List webhooks",
				},
			},
		},
		{
			Pattern: "events/webhooks/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeString,
					Description: "The name of the webhook.",
					Required:    true,
				},
				"format": {
					Type:        framework.TypeString,
					Description: "Format of event payload. If not set, defaults to cloudevents.",
					Default:     "cloudevents",
				},
				"pattern": {
					Type:        framework.TypeString,
					Description: "Pattern to filter which event types should be sent. Can use * for glob-style wildcard matching.",
					Required:    true,
				},
				"url": {
					Type:        framework.TypeString,
					Description: "URL to POST events to.",
					Required:    true,
					DisplayAttrs: &framework.DisplayAttributes{
						// Webhook URLs, such as Slack's, may have secrets in them.
						Sensitive: true,
					},
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ReadOperation: &framework.PathOperation{
					Callback: b.handleReadWebhook,
					Responses: map[int][]framework.Response{
						http.StatusOK: {{
							Description: "OK",
						}},
					},
					Summary: "Read the configuration for a webhook.",
				},
				logical.UpdateOperation: &framework.PathOperation{
					Callback: b.handleUpdateWebhook,
					Responses: map[int][]framework.Response{
						http.StatusNoContent: {{
							Description: "OK",
						}},
					},
					Summary: "Create or update a webhook.",
				},
				logical.DeleteOperation: &framework.PathOperation{
					Callback: b.handleDeleteWebhook,
					Responses: map[int][]framework.Response{
						http.StatusNoContent: {{
							Description: "OK",
						}},
					},
					Summary: "Delete a webhook.",
				},
			},
		},
	}
}

func (b *SystemBackend) handleListWebhooks(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	return logical.ErrorResponse("Not implemented"), nil
}

func (b *SystemBackend) handleUpdateWebhook(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	urlRaw, ok := d.GetOk("url")
	if !ok {
		return logical.ErrorResponse("'url' is required, to specify where to send events to"), nil
	}
	patternRaw, ok := d.GetOk("pattern")
	if !ok {
		return logical.ErrorResponse("'pattern' is required, to specify which events to send"), nil
	}

	const (
		cloudEvents = "cloudevents"
		slack       = "slack"
	)
	var format string
	formatRaw, ok := d.GetOk("format")
	if ok {
		format = formatRaw.(string)
	} else {
		format = cloudEvents
	}
	if format != cloudEvents && format != slack {
		return logical.ErrorResponse("'format' must be unset, or one of %q or %q, but was %q", cloudEvents, slack, formatRaw.(string)), nil
	}

	ns, err := namespace.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace from context: %w", err)
	}

	if err := b.Core.events.RegisterWebhook(ctx, ns, urlRaw.(string), patternRaw.(string), format); err != nil {
		return nil, fmt.Errorf("failed to register webhook: %w", err)
	}

	return nil, nil
}

func (b *SystemBackend) handleReadWebhook(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	return logical.ErrorResponse("Not implemented"), nil
}

func (b *SystemBackend) handleDeleteWebhook(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	return logical.ErrorResponse("Not implemented"), nil
}
