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
	"os"
	"text/tabwriter"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"github.com/AlecAivazis/survey/v2"
	"github.com/atotto/clipboard"
	"github.com/lekkodev/cli/pkg/apikey"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func apikeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "api key management",
	}
	cmd.AddCommand(
		createAPIKeyCmd(),
		listAPIKeysCmd(),
		checkAPIKeyCmd(),
		deleteAPIKeyCmd(),
		showAPIKeyCmd(),
		copyAPIKeyCmd(),
	)
	return cmd
}

func createAPIKeyCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an api key",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if len(rs.GetLekkoAPIKey()) > 0 {
				fmt.Printf("Found existing API key, use %s or %s to retrieve it.\n", logging.Bold("lekko apikey show"), logging.Bold("lekko apikey copy"))
				fmt.Println("Generating a new API key will overwrite the existing one.")
				doIt := false
				if err := survey.AskOne(&survey.Confirm{
					Message: "Continue?",
					Default: false,
				}, &doIt); err != nil {
					return err
				}
				if !doIt {
					return errors.New("Aborted!")
				}
			}
			a := apikey.NewAPIKey(lekko.NewBFFClient(rs))
			if len(name) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Name (optional):",
					Help:    "Name to give the api key",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			fmt.Printf("Generating api key for team '%s'...\n", rs.GetLekkoTeam())
			resp, err := a.Create(cmd.Context(), rs.GetLekkoTeam(), name)
			if err != nil {
				return err
			}
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				ws.SetLekkoAPIKey(resp.GetApiKey())
				return nil
			}, secrets.RequireLekko()); err != nil {
				return errors.Wrap(err, "write API key to secrets")
			}
			fmt.Printf("Generated api key named '%s':\n\t%s\n", resp.GetNickname(), logging.Bold(resp.GetApiKey()))
			fmt.Printf("Use %s command to copy the API key to your clipboard\n", logging.Bold("lekko apikey copy"))
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name to give the new api key")
	return cmd
}

func listAPIKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all api keys for the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			a := apikey.NewAPIKey(lekko.NewBFFClient(rs))
			keys, err := a.List(cmd.Context(), rs.GetLekkoTeam())
			if err != nil {
				return errors.Wrap(err, "list")
			}
			printAPIKeys(keys...)
			return nil
		},
	}
	return cmd
}

func printAPIKeys(keys ...*bffv1beta1.APIKey) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "Team\tName\tCreated By\tCreated At\n")
	for _, key := range keys {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key.TeamName, key.Nickname, key.CreatedBy, key.CreatedAt.AsTime())
	}
	w.Flush()
}

func checkAPIKeyCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check an api key to ensure it can be used to authenticate with lekko",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			a := apikey.NewAPIKey(lekko.NewBFFClient(rs))
			if len(key) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "API Key:",
					Help:    "API Key to check authentication for",
				}, &key); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			fmt.Printf("Checking authentication status for api key in team '%s'...\n", rs.GetLekkoTeam())
			lekkoKey, err := a.Check(cmd.Context(), key)
			if err != nil {
				fmt.Printf("Lekko: Unauthenticated %s\n", logging.Red("✖"))
				return errors.Wrap(err, "check")
			}
			fmt.Printf("Lekko: Authenticated %s\n", logging.Green("✔"))
			printAPIKeys(lekkoKey)
			return nil
		},
	}
	cmd.Flags().StringVarP(&key, "key", "k", "", "api key to check authentication for")
	return cmd
}

func deleteAPIKeyCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an api key",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			a := apikey.NewAPIKey(lekko.NewBFFClient(rs))
			if len(name) == 0 {
				keys, err := a.List(cmd.Context(), rs.GetLekkoTeam())
				if err != nil {
					return errors.Wrap(err, "list")
				}
				var options []string
				for _, key := range keys {
					options = append(options, key.GetNickname())
				}
				if err := survey.AskOne(&survey.Select{
					Message: "API Key to delete:",
					Options: options,
					Help:    "Name of api key to delete",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			fmt.Printf("Deleting api key '%s' in team '%s'...\n", name, rs.GetLekkoTeam())
			if err := confirmInput(name); err != nil {
				return err
			}
			if err := a.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Printf("Deleted api key.\n")
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name of api key to delete")
	return cmd
}

func showAPIKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the locally stored API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			fmt.Println(rs.GetLekkoAPIKey())
			return nil
		},
	}
	return cmd
}

func copyAPIKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy the locally stored API key to the clipboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if err := clipboard.WriteAll(rs.GetLekkoAPIKey()); err != nil {
				return errors.Wrap(err, "copy")
			}
			fmt.Println("API key copied to clipboard")
			return nil
		},
	}
	return cmd
}
