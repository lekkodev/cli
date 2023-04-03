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

package repo

import (
	"context"

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/bufbuild/connect-go/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	connect_go "github.com/bufbuild/connect-go"
	"github.com/pkg/errors"
)

// Responsible for all repository management actions on the command line.
// e.g. create/delete/list repos.
type RepoCmd struct {
	lekkoBFFClient bffv1beta1connect.BFFServiceClient
}

func NewRepoCmd(bff bffv1beta1connect.BFFServiceClient) *RepoCmd {
	return &RepoCmd{
		lekkoBFFClient: bff,
	}
}

type Repository struct {
	Owner       string
	RepoName    string
	Description string
	URL         string
}

func (r *RepoCmd) List(ctx context.Context) ([]*Repository, error) {
	resp, err := r.lekkoBFFClient.ListRepositories(ctx, connect_go.NewRequest(&bffv1beta1.ListRepositoriesRequest{}))
	if err != nil {
		return nil, errors.Wrap(err, "list repos")
	}
	var ret []*Repository
	for _, r := range resp.Msg.GetRepositories() {
		ret = append(ret, repoFromProto(r))
	}
	return ret, nil
}

func repoFromProto(repo *bffv1beta1.Repository) *Repository {
	if repo == nil {
		return nil
	}
	return &Repository{
		Owner:       repo.OwnerName,
		RepoName:    repo.RepoName,
		Description: repo.Description,
		URL:         repo.Url,
	}
}

func (r *RepoCmd) Create(ctx context.Context, owner, repo string) (string, error) {
	resp, err := r.lekkoBFFClient.CreateRepository(ctx, connect_go.NewRequest(&bffv1beta1.CreateRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repo,
		},
	}))
	if err != nil {
		return "", errors.Wrap(err, "create repository")
	}
	return resp.Msg.GetUrl(), nil
}

func (r *RepoCmd) Delete(ctx context.Context, owner, repo string, deleteOnRemote bool) error {
	_, err := r.lekkoBFFClient.DeleteRepository(ctx, connect_go.NewRequest(&bffv1beta1.DeleteRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repo,
		},
		DeleteOnRemote: deleteOnRemote,
	}))
	if err != nil {
		return errors.Wrap(err, "delete repository")
	}
	return nil
}
