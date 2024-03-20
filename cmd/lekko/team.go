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
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"os"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/team"
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
		teamListCmd(),
		teamSwitchCmd(),
		createCmd(),
		addMemberCmd(),
		removeMemberCmd(),
		teamListMembersCmd(),
		deleteTeamCmd(),
	)
	return cmd
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the team currently in use",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail(secrets.RequireLekkoToken())
		t := team.NewTeam(lekko.NewBFFClient(rs))
		fmt.Println(t.Show(rs))
		return nil
	},
}

func teamListCmd() *cobra.Command {
	var flagOutput string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list the teams that the logged-in user is a member of",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekkoToken())
			t := team.NewTeam(lekko.NewBFFClient(rs))
			memberships, err := t.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(memberships) == 0 {
				fmt.Printf("User '%s' has no team memberhips\n", rs.GetLekkoUsername())
				return nil
			}
			printTeamMemberships(memberships, flagOutput)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "table", "Output format. ['json', 'table']")
	return cmd
}

func teamSwitchCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "switch the team currently in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				t := team.NewTeam(lekko.NewBFFClient(ws))
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
				if err := t.Use(ctx, name, ws); err != nil {
					return err
				}
				return nil
			}, secrets.RequireLekkoToken()); err != nil {
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
					Message: "Team Name:",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				return team.NewTeam(lekko.NewBFFClient(ws)).Create(cmd.Context(), name, ws)
			}, secrets.RequireLekkoToken()); err != nil {
				return err
			}
			fmt.Printf("Successfully created team %s, and switched to it", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of team to create")
	return cmd
}

func addMemberCmd() *cobra.Command {
	var email string
	var role team.MemberRole
	cmd := &cobra.Command{
		Use:   "add-member",
		Short: "add an existing lekko user as a member to the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email to add:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
				if _, err := mail.ParseAddress(email); err != nil {
					return errors.Wrap(err, "invalid email")
				}
			}
			if len(role) == 0 {
				var roleStr string
				if err := survey.AskOne(&survey.Select{
					Message: "Role:",
					Options: []string{string(team.MemberRoleMember), string(team.MemberRoleOwner)},
				}, &roleStr); err != nil {
					return errors.Wrap(err, "prompt")
				}
				role = team.MemberRole(roleStr)
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if err := team.NewTeam(lekko.NewBFFClient(rs)).AddMember(cmd.Context(), email, role); err != nil {
				return errors.Wrap(err, "add member")
			}
			fmt.Printf("User %s added as %s to team %s", email, role, rs.GetLekkoTeam())
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email of existing lekko user to add")
	cmd.Flags().VarP(&role, "role", "r", "role to give member. allowed: 'owner', 'member'.")
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
					Message: "Email to remove:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if err := team.NewTeam(lekko.NewBFFClient(rs)).RemoveMember(cmd.Context(), email); err != nil {
				return errors.Wrap(err, "remove member")
			}
			fmt.Printf("User %s removed from team %s", email, rs.GetLekkoTeam())
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email of existing lekko user to add")
	return cmd
}

func teamListMembersCmd() *cobra.Command {
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "list-members",
		Short: "list the members of the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			t := team.NewTeam(lekko.NewBFFClient(rs))
			memberships, err := t.ListMemberships(cmd.Context())
			if err != nil {
				return err
			}
			if len(memberships) == 0 {
				fmt.Printf("Team '%s' has no memberhips\n", rs.GetLekkoTeam())
				return nil
			}
			printTeamMemberships(memberships, flagOutput)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "table", "Output format. ['json', 'table']")
	return cmd
}

func printTeamMemberships(memberships []*team.TeamMembership, output string) {
	switch output {
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "Team Name\tEmail\tRole\tStatus\n")
		for _, m := range memberships {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.TeamName, m.User, m.Role, m.UserStatus)
		}
		w.Flush()
	case "json":
		b, err := json.MarshalIndent(memberships, "", "    ")
		if err != nil {
			log.Fatalf("unable to print team memberships: %v", err)
		}
		fmt.Println(string(b))
	default:
		fmt.Printf("unknown output format: %s", output)
	}
}

func deleteTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "delete the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())

			fmt.Printf("Deleting team '%s' from Lekko...\n", rs.GetLekkoTeam())
			if err := confirmInput(rs.GetLekkoTeam()); err != nil {
				return err
			}

			t := team.NewTeam(lekko.NewBFFClient(rs))
			if err := t.Delete(cmd.Context(), rs.GetLekkoTeam()); err != nil {
				return err
			}
			secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				ws.SetLekkoTeam("")
				return nil
			})
			fmt.Printf("Team '%s' deleted\n", rs.GetLekkoTeam())
			return nil
		},
	}
	return cmd
}
