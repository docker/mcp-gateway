// Copyright 2025-2026 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolver

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	"github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/secrets"
)

var _ resolverv1connect.ResolverServiceHandler = &resolverService{}

type resolverService struct {
	resolver secrets.Resolver
}

func NewResolverHandler(r secrets.Resolver) resolverv1connect.ResolverServiceHandler {
	return &resolverService{r}
}

func (r resolverService) GetSecrets(ctx context.Context, c *connect.Request[resolverv1.GetSecretsRequest]) (*connect.Response[resolverv1.GetSecretsResponse], error) {
	msgPattern := c.Msg.GetPattern()
	pattern, err := secrets.ParsePattern(msgPattern)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pattern %q: %w", msgPattern, err))
	}

	envelopes, err := r.resolver.GetSecrets(ctx, pattern)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, secrets.ErrNotFound)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get secret %q: %w", msgPattern, err))
	}
	if len(envelopes) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, secrets.ErrNotFound)
	}
	var items []*resolverv1.GetSecretsResponse_Envelope
	for _, envelope := range envelopes {
		items = append(items, resolverv1.GetSecretsResponse_Envelope_builder{
			Id:         proto.String(envelope.ID.String()),
			Value:      envelope.Value,
			Metadata:   envelope.Metadata,
			Provider:   proto.String(envelope.Provider),
			Version:    proto.String(envelope.Version),
			CreatedAt:  timestamppb.New(envelope.CreatedAt),
			ResolvedAt: timestamppb.New(envelope.ResolvedAt),
			ExpiresAt:  timestamppb.New(envelope.ExpiresAt),
		}.Build())
	}
	return connect.NewResponse(resolverv1.GetSecretsResponse_builder{
		Envelopes: items,
	}.Build()), nil
}

var _ secrets.Resolver = &resolverClient{}

type resolverClient struct {
	resolverClient resolverv1connect.ResolverServiceClient
}

func NewResolverClient(httpClient connect.HTTPClient) secrets.Resolver {
	return &resolverClient{resolverClient: resolverv1connect.NewResolverServiceClient(httpClient, "http://unix")}
}

func (r resolverClient) GetSecrets(ctx context.Context, pattern secrets.Pattern) ([]secrets.Envelope, error) {
	req := connect.NewRequest(resolverv1.GetSecretsRequest_builder{
		Pattern: proto.String(pattern.String()),
	}.Build())
	resp, err := r.resolverClient.GetSecrets(ctx, req)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			err = secrets.ErrNotFound
		}
		return nil, err
	}

	var envelopes []secrets.Envelope
	for _, item := range resp.Msg.GetEnvelopes() {
		id, err := secrets.ParseID(item.GetId())
		if err != nil {
			continue
		}
		envelopes = append(envelopes, secrets.Envelope{
			ID:         id,
			Value:      item.GetValue(),
			Metadata:   item.GetMetadata(),
			Provider:   item.GetProvider(),
			Version:    item.GetVersion(),
			CreatedAt:  item.GetCreatedAt().AsTime(),
			ResolvedAt: item.GetResolvedAt().AsTime(),
			ExpiresAt:  item.GetExpiresAt().AsTime(),
		})
	}
	return envelopes, nil
}
