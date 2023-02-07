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

package main

import (
	"fmt"
	"net/mail"
	"os"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/team"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func teamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "team management",
	}
	cmd.AddCommand(
		showCmd,
		teamListCmd,
		teamSwitchCmd(),
		createCmd(),
		addMemberCmd(),
		removeMemberCmd(),
		teamListMembersCmd,
	)
	return cmd
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the team currently in use",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := team.NewTeam(metadata.NewSecretsOrFail())
		fmt.Println(t.Show())
		return nil
	},
}

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "list the teams that the logged-in user is a member of",
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets := metadata.NewSecretsOrFail()
		t := team.NewTeam(secrets)
		memberships, err := t.List(cmd.Context())
		if err != nil {
			return err
		}
		if len(memberships) == 0 {
			fmt.Printf("User '%s' has no team memberhips\n", secrets.GetLekkoUsername())
			return nil
		}
		printTeamMemberships(memberships)
		return nil
	},
}

func teamSwitchCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "switch the team currently in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets := metadata.NewSecretsOrFail()
			defer secrets.Close() // since we are mutating the team
			t := team.NewTeam(secrets)
			ctx := cmd.Context()
			if len(name) == 0 {
				memberships, err := t.List(ctx)
				if err != nil {
					return err
				}
				var options []string
				for _, m := range memberships {
					options = append(options, m.TeamName)
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Choose a team:",
					Options: options,
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if err := t.Use(ctx, name); err != nil {
				return err
			}
			fmt.Printf("Switched team to '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of team to switch to")
	return cmd
}

func createCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a lekko team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(name) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Team Name: ",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			secrets := metadata.NewSecretsOrFail()
			defer secrets.Close() // since we are mutating the team
			return team.NewTeam(secrets).Create(cmd.Context(), name)
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of team to create")
	return cmd
}

func addMemberCmd() *cobra.Command {
	var email string
	var owner bool
	cmd := &cobra.Command{
		Use:   "add-member",
		Short: "add an existing lekko user as a member to the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email to add: ",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
				if _, err := mail.ParseAddress(email); err != nil {
					return errors.Wrap(err, "invalid email")
				}
			}

			secrets := metadata.NewSecretsOrFail()
			role := team.MemberRoleMember
			if owner {
				role = team.MemberRoleOwner
			}
			if err := team.NewTeam(secrets).AddMember(cmd.Context(), email, role); err != nil {
				return errors.Wrap(err, "add member")
			}
			fmt.Printf("User %s added as %s to team %s", email, role, secrets.GetLekkoTeam())
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email of existing lekko user to add")
	cmd.Flags().BoolVarP(&owner, "owner", "o", false, "give the user admin privileges")
	return cmd
}

func removeMemberCmd() *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "remove-member",
		Short: "remove a member from the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email to remove: ",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			secrets := metadata.NewSecretsOrFail()
			if err := team.NewTeam(secrets).RemoveMember(cmd.Context(), email); err != nil {
				return errors.Wrap(err, "remove member")
			}
			fmt.Printf("User %s removed from team %s", email, secrets.GetLekkoTeam())
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email of existing lekko user to add")
	return cmd
}

var teamListMembersCmd = &cobra.Command{
	Use:   "list-members",
	Short: "list the members of the currently active team",
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets := metadata.NewSecretsOrFail()
		t := team.NewTeam(secrets)
		memberships, err := t.ListMemberships(cmd.Context())
		if err != nil {
			return err
		}
		if len(memberships) == 0 {
			fmt.Printf("Team '%s' has no memberhips\n", secrets.GetLekkoTeam())
			return nil
		}
		printTeamMemberships(memberships)
		return nil
	},
}

func printTeamMemberships(memberships []*team.TeamMembership) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "Team Name\tEmail\tRole\n")
	for _, m := range memberships {
		fmt.Fprintf(w, "%s\t%s\t%s\n", m.TeamName, m.User, m.Role)
	}
	w.Flush()
}
