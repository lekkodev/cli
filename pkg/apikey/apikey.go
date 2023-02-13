// Copyright 2022 Lekko Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apikey

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/pkg/errors"
)

// Provides functionality around interacting with api keys.
type APIKey struct {
	bff bffv1beta1connect.BFFServiceClient
}

func NewAPIKey(bff bffv1beta1connect.BFFServiceClient) *APIKey {
	return &APIKey{bff: bff}
}

func (a *APIKey) Create(ctx context.Context, nickname string) (string, error) {
	resp, err := a.bff.GenerateAPIKey(ctx, connect.NewRequest(&bffv1beta1.GenerateAPIKeyRequest{
		Nickname: nickname,
	}))
	if err != nil {
		return "", errors.Wrap(err, "generate api key")
	}
	return resp.Msg.GetApiKey(), nil
}

func (a *APIKey) List(ctx context.Context) ([]*bffv1beta1.APIKey, error) {
	resp, err := a.bff.ListAPIKeys(ctx, connect.NewRequest(&bffv1beta1.ListAPIKeysRequest{}))
	if err != nil {
		return nil, errors.Wrap(err, "list api keys")
	}
	return resp.Msg.GetApiKeys(), nil
}

func (a *APIKey) Check(ctx context.Context, apikey string) (*bffv1beta1.APIKey, error) {
	resp, err := a.bff.CheckAPIKey(ctx, connect.NewRequest(&bffv1beta1.CheckAPIKeyRequest{
		ApiKey: apikey,
	}))
	if err != nil {
		return nil, errors.Wrap(err, "check api key")
	}
	return resp.Msg.GetKey(), nil
}

func (a *APIKey) Delete(ctx context.Context, nickname string) error {
	_, err := a.bff.DeleteAPIKey(ctx, connect.NewRequest(&bffv1beta1.DeleteAPIKeyRequest{
		Nickname: nickname,
	}))
	return err
}
