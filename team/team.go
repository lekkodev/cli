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

package team

import (
	"context"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/lekkodev/cli/lekko"
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

type TeamStore interface {
	GetLekkoTeam() string
	SetLekkoTeam(team string)
	Close() error
}

// Team is responsible for team management with Lekko
type Team struct {
	teamStore      TeamStore
	lekkoBFFClient bffv1beta1connect.BFFServiceClient
}

func NewTeam(secrets metadata.Secrets) *Team {
	return &Team{
		teamStore:      secrets,
		lekkoBFFClient: lekko.NewBFFClient(secrets),
	}
}

func (t *Team) Show() string {
	return t.teamStore.GetLekkoTeam()
}

func (t *Team) Use(ctx context.Context, team string) error {
	if _, err := t.lekkoBFFClient.UseTeam(ctx, connect.NewRequest(&bffv1beta1.UseTeamRequest{
		Name: team,
	})); err != nil {
		return errors.Wrap(err, "use team")
	}
	t.teamStore.SetLekkoTeam(team)
	return nil
}

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleMember MemberRole = "member"
)

type TeamMembership struct {
	TeamName string
	User     string
	Role     string
}

func (t *Team) List(ctx context.Context) ([]*TeamMembership, error) {
	resp, err := t.lekkoBFFClient.ListUserMemberships(ctx, connect.NewRequest(&bffv1beta1.ListUserMembershipsRequest{}))
	if err != nil {
		return nil, errors.Wrap(err, "list team memberships")
	}
	var ret []*TeamMembership
	for _, m := range resp.Msg.GetMemberships() {
		ret = append(ret, teamMembershipFromProto(m))
	}
	return ret, nil
}

func teamMembershipFromProto(m *bffv1beta1.Membership) *TeamMembership {
	if m == nil {
		return nil
	}
	return &TeamMembership{
		TeamName: m.GetTeamName(),
		User:     m.GetUsername(),
		Role:     roleFromProto(m.GetRole()),
	}
}

func roleFromProto(role bffv1beta1.MembershipRole) string {
	parts := strings.Split(role.String(), "_")
	return strings.ToLower(parts[len(parts)-1])
}

func roleToProto(role MemberRole) bffv1beta1.MembershipRole {
	switch role {
	case MemberRoleOwner:
		return bffv1beta1.MembershipRole_MEMBERSHIP_ROLE_OWNER
	case MemberRoleMember:
		return bffv1beta1.MembershipRole_MEMBERSHIP_ROLE_MEMBER
	default:
		return bffv1beta1.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED
	}
}

func (t *Team) Create(ctx context.Context, name string) error {
	if _, err := t.lekkoBFFClient.CreateTeam(ctx, connect.NewRequest(&bffv1beta1.CreateTeamRequest{
		Name: name,
	})); err != nil {
		return errors.Wrap(err, "create team")
	}
	t.teamStore.SetLekkoTeam(name)
	return nil
}

func (t *Team) AddMember(ctx context.Context, email string, role MemberRole) error {
	if _, err := t.lekkoBFFClient.UpsertMembership(ctx, connect.NewRequest(&bffv1beta1.UpsertMembershipRequest{
		Username: email,
		Role:     roleToProto(role),
	})); err != nil {
		return errors.Wrap(err, "upsert membership")
	}
	return nil
}

func (t *Team) ListMemberships(ctx context.Context) ([]*TeamMembership, error) {
	resp, err := t.lekkoBFFClient.ListTeamMemberships(ctx, connect.NewRequest(&bffv1beta1.ListTeamMembershipsRequest{}))
	if err != nil {
		return nil, errors.Wrap(err, "list team memberships")
	}
	var ret []*TeamMembership
	for _, m := range resp.Msg.GetMemberships() {
		ret = append(ret, teamMembershipFromProto(m))
	}
	return ret, nil
}
