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
	"strings"
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
		Short: "Team management",
	}
	cmd.AddCommand(
		teamShowCmd(),
		teamListCmd(),
		teamSwitchCmd(),
		teamCreateCmd(),
		teamAddMemberCmd(),
		teamRemoveMemberCmd(),
		teamListMembersCmd(),
	)
	return cmd
}

func teamShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Short:                 "Show the team currently in use",
		Use:                   formCmdUse("show", FlagOptions),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{}, args); err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekkoToken())
			t := team.NewTeam(lekko.NewBFFClient(rs))
			printLinef(cmd, "%s\n", t.Show(rs))
			return nil
		},
	}
	cmd.Flags().BoolP(QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func teamListCmd() *cobra.Command {
	var isQuiet bool
	var flagOutput string
	cmd := &cobra.Command{
		Short:                 "List the teams that the logged-in user is a member of",
		Use:                   formCmdUse("list", FlagOptions),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{}, args); err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekkoToken())
			t := team.NewTeam(lekko.NewBFFClient(rs))
			memberships, err := t.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(memberships) == 0 {
				if !IsInteractive {
					fmt.Printf("User '%s' has no team memberhips\n", rs.GetLekkoUsername())
				}
				return nil
			}
			printTeamMemberships(cmd, memberships, flagOutput)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "table", "output format: ['json', 'table']")
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func teamSwitchCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var name string
	cmd := &cobra.Command{
		Short:                 "Switch the team currently in use",
		Use:                   formCmdUse("switch", "name"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"name"}, args); err != nil {
				return err
			}
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				t := team.NewTeam(lekko.NewBFFClient(ws))
				ctx := cmd.Context()

				rArgs, _ := getNArgs(1, args)
				name = rArgs[0]

				memberships, err := t.List(ctx)
				if err != nil {
					return err
				}
				if len(name) == 0 {
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
				} else {
					isTeam := false
					for _, tm := range memberships {
						if tm.TeamName == name {
							isTeam = true
							break
						}
					}
					if !isTeam {
						return errors.New(" No team with the given name found")
					}
				}
				if !isDryRun {
					if err := t.Use(ctx, name, ws); err != nil {
						return err
					}
				}
				return nil
			}, secrets.RequireLekkoToken()); err != nil {
				return err
			}

			if !isQuiet {
				printLinef(cmd, "Switched team to '%s'\n", name)
			} else {
				fmt.Printf("%s", name)
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	return cmd
}

func teamCreateCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var name string
	cmd := &cobra.Command{
		Short:                 "Create a lekko team",
		Use:                   formCmdUse("create", "name"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"name"}, args); err != nil {
				return err
			}
			rArgs, _ := getNArgs(1, args)
			name = rArgs[0]

			if len(name) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Team Name:",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if !isDryRun {
				if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					return team.NewTeam(lekko.NewBFFClient(ws)).Create(cmd.Context(), name, ws)
				}, secrets.RequireLekkoToken()); err != nil {
					return err
				}
			}
			if !isQuiet {
				printLinef(cmd, "Created team %s, and switched to it \n", name)
			} else {
				fmt.Printf("%s", name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	return cmd
}

func teamAddMemberCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var email string
	var role team.MemberRole
	cmd := &cobra.Command{
		Short:                 "Add an existing lekko user as a member to the currently active team",
		Use:                   formCmdUse("add-member", "email", "role"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"email", "role"}, args); err != nil {
				return err
			}
			rArgs, _ := getNArgs(2, args)

			email = rArgs[0]
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email to add:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if _, err := mail.ParseAddress(email); err != nil {
				return errors.Wrap(err, "invalid email")
			}
			roleStr := rArgs[1]
			if len(roleStr) == 0 {
				//var roleStr string
				if err := survey.AskOne(&survey.Select{
					Message: "Role:",
					Options: []string{string(team.MemberRoleMember), string(team.MemberRoleOwner)},
				}, &roleStr); err != nil {
					return errors.Wrap(err, "prompt")
				}
				role = team.MemberRole(roleStr)
			} else {
				role = team.MemberRole(roleStr)
				if role != team.MemberRoleOwner && role != team.MemberRoleMember {
					return errors.New("unrecognized team role")
				}
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())

			if !isDryRun {
				if err := team.NewTeam(lekko.NewBFFClient(rs)).AddMember(cmd.Context(), email, role); err != nil {
					return errors.Wrap(err, "add member")
				}
			}

			if !isQuiet {
				printLinef(cmd, "User %s added as %s to team %s\n", email, role, rs.GetLekkoTeam())
			} else {
				printLinef(cmd, "%s", email)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	return cmd
}

func teamRemoveMemberCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var email string
	cmd := &cobra.Command{
		Short:                 "Remove a member from the currently active team",
		Use:                   formCmdUse("remove-member", "email"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"email"}, args); err != nil {
				return err
			}
			rArgs, _ := getNArgs(1, args)
			email = rArgs[0]
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email to remove:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if !isDryRun {
				if err := team.NewTeam(lekko.NewBFFClient(rs)).RemoveMember(cmd.Context(), email); err != nil {
					return errors.Wrap(err, "remove member")
				}
			}
			if !isQuiet {
				printLinef(cmd, "User %s removed from team %s\n", email, rs.GetLekkoTeam())
			} else {
				printLinef(cmd, "%s", email)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	return cmd
}

func teamListMembersCmd() *cobra.Command {
	var flagOutput string
	var isQuiet bool
	cmd := &cobra.Command{
		Short:                 "List the members of the currently active team",
		Use:                   formCmdUse("list-members"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			t := team.NewTeam(lekko.NewBFFClient(rs))
			memberships, err := t.ListMemberships(cmd.Context())
			if err != nil {
				return err
			}
			if len(memberships) == 0 {
				if !isQuiet {
					fmt.Printf("Team '%s' has no memberhips\n", rs.GetLekkoTeam())
				}
				return nil
			}

			printTeamMemberships(cmd, memberships, flagOutput)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "table", "output format: ['json', 'table']")
	return cmd
}

func printTeamMemberships(cmd *cobra.Command, memberships []*team.TeamMembership, output string) {
	if isQuiet, _ := cmd.Flags().GetBool(QuietModeFlag); isQuiet {
		out := ""
		for _, m := range memberships {
			switch cmd.Name() {
			case "list-members":
				out += fmt.Sprintf("%s ", m.User)
			case "list":
				out += fmt.Sprintf("%s ", m.TeamName)
			}
		}
		out = strings.TrimSpace(out)
		printLinef(cmd, "%s", out)
	} else {
		switch output {
		case "table":
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "Team Name\tEmail\tRole\tStatus\n")
			for _, m := range memberships {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.TeamName, m.User, m.Role, m.UserStatus)
			}
			_ = w.Flush()
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
}
