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

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/connectrpc/go/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"connectrpc.com/connect"

	"github.com/pkg/errors"
)

// Provides functionality around interacting with api keys.
type APIKeyManager struct {
	bff bffv1beta1connect.BFFServiceClient
}

func NewAPIKey(bff bffv1beta1connect.BFFServiceClient) *APIKeyManager {
	return &APIKeyManager{bff: bff}
}

func (a *APIKeyManager) Create(ctx context.Context, teamname, nickname string) (*bffv1beta1.GenerateAPIKeyResponse, error) {
	resp, err := a.bff.GenerateAPIKey(ctx, connect.NewRequest(&bffv1beta1.GenerateAPIKeyRequest{
		Nickname: nickname,
		TeamName: teamname,
	}))
	if err != nil {
		return nil, errors.Wrap(err, "generate api key")
	}
	return resp.Msg, nil
}

func (a *APIKeyManager) List(ctx context.Context, teamname string) ([]*bffv1beta1.APIKey, error) {
	resp, err := a.bff.ListAPIKeys(ctx, connect.NewRequest(&bffv1beta1.ListAPIKeysRequest{TeamName: teamname}))
	if err != nil {
		return nil, errors.Wrap(err, "list api keys")
	}
	return resp.Msg.GetApiKeys(), nil
}

func (a *APIKeyManager) Check(ctx context.Context, apikey string) (*bffv1beta1.APIKey, error) {
	resp, err := a.bff.CheckAPIKey(ctx, connect.NewRequest(&bffv1beta1.CheckAPIKeyRequest{
		ApiKey: apikey,
	}))
	if err != nil {
		return nil, errors.Wrap(err, "check api key")
	}
	return resp.Msg.GetKey(), nil
}

func (a *APIKeyManager) Delete(ctx context.Context, nickname string) error {
	_, err := a.bff.DeleteAPIKey(ctx, connect.NewRequest(&bffv1beta1.DeleteAPIKeyRequest{
		Nickname: nickname,
	}))
	return err
}
