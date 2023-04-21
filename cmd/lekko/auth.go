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
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/oauth"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "authenticates lekko cli",
	}

	cmd.AddCommand(confirmUserCmd())
	cmd.AddCommand(confirmForgotPasswordCmd())
	cmd.AddCommand(forgotPasswordCmd())
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(registerCmd())
	cmd.AddCommand(resendVerification())
	cmd.AddCommand(statusCmd)
	cmd.AddCommand(tokensCmd)

	return cmd
}

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "authenticate with lekko and github, if unauthenticated",
		RunE: func(cmd *cobra.Command, args []string) error {
			return secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Login(cmd.Context(), ws)
			})
		},
	}
}

func logoutCmd() *cobra.Command {
	var p provider
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "log out of lekko or github",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(string(p)) == 0 {
				return errors.Errorf("provider must be specified")
			}
			return secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Logout(cmd.Context(), string(p), ws)
			})
		},
	}
	cmd.Flags().VarP(&p, "provider", "p", "provider to log out. allowed: 'lekko', 'github'.")
	return cmd
}

func registerCmd() *cobra.Command {
	var email, password, confirmPassword string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "register an account with lekko",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt email")
				}
			}
			if _, err := mail.ParseAddress(email); err != nil {
				return errors.New("invalid email address")
			}
			// prompt password
			if err := survey.AskOne(&survey.Password{
				Message: "Password:",
			}, &password); err != nil {
				return errors.Wrap(err, "prompt password")
			}
			if err := survey.AskOne(&survey.Password{
				Message: "Confirm Password:",
			}, &confirmPassword); err != nil {
				return errors.Wrap(err, "prompt confirm password")
			}

			if password != confirmPassword {
				return errors.New("passwords don't match")
			}
			auth := oauth.NewOAuth(lekko.NewBFFClient(secrets.NewSecretsOrFail()))
			if err := auth.Register(cmd.Context(), email, password, confirmPassword); err != nil {
				return err
			}
			fmt.Println("Run `lekko auth confirm` to confirm account.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email to create lekko account with")
	return cmd
}

func confirmUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confirm",
		Short: "confirm a new user account",
		RunE: func(cmd *cobra.Command, args []string) error {
			var email, code string
			rs := secrets.NewSecretsOrFail()
			if err := survey.AskOne(&survey.Input{
				Message: "Email:",
			}, &email); err != nil {
				return errors.Wrap(err, "prompt email")
			}

			if err := survey.AskOne(&survey.Input{
				Message: "Verification Code:",
			}, &code); err != nil {
				return errors.Wrap(err, "prompt code")
			}
			if err := oauth.NewOAuth(lekko.NewBFFClient(rs)).ConfirmUser(cmd.Context(), email, code); err != nil {
				return err
			}
			fmt.Println("Account registered with lekko.")
			fmt.Println("Run `lekko auth login` to complete oauth.")
			return nil
		},
	}
	return cmd
}

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "display token(s) currently in use",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail()
		tokens := oauth.NewOAuth(lekko.NewBFFClient(rs)).Tokens(cmd.Context(), rs)
		fmt.Println(strings.Join(tokens, "\n"))
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "display lekko authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail()
		auth := oauth.NewOAuth(lekko.NewBFFClient(rs))
		auth.Status(cmd.Context(), false, rs)
		return nil
	},
}

func forgotPasswordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forgot-password",
		Short: "Forgot password. The email must be verified in order to receive the password reset email.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var email string
			rs := secrets.NewSecretsOrFail()
			if err := survey.AskOne(&survey.Input{
				Message: "Email:",
			}, &email); err != nil {
				return errors.Wrap(err, "prompt email")
			}
			if err := oauth.NewOAuth(lekko.NewBFFClient(rs)).ForgotPassword(cmd.Context(), email); err != nil {
				return err
			}
			fmt.Printf("An email with a verification code has been sent to %s\n", email)
			fmt.Println("Run `lekko auth forgot-password-confirm` to complete password reset.")
			return nil
		},
	}
	return cmd
}

func confirmForgotPasswordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forgot-password-confirm",
		Short: "Confirm forgot password",
		RunE: func(cmd *cobra.Command, args []string) error {
			var email, newPassword, confirmNewPassword, verificationCode string
			rs := secrets.NewSecretsOrFail()
			if err := survey.AskOne(&survey.Input{
				Message: "Email:",
			}, &email); err != nil {
				return errors.Wrap(err, "prompt email")
			}
			if err := survey.AskOne(&survey.Password{
				Message: "New Password:",
			}, &newPassword); err != nil {
				return errors.Wrap(err, "prompt new password")
			}
			if err := survey.AskOne(&survey.Password{
				Message: "Confirm New Password:",
			}, &confirmNewPassword); err != nil {
				return errors.Wrap(err, "prompt confirm password")
			}
			if newPassword != confirmNewPassword {
				return errors.New("passwords do not match")
			}
			if err := survey.AskOne(&survey.Input{
				Message: "Verification Code:",
			}, &verificationCode); err != nil {
				return errors.Wrap(err, "prompt verification code")
			}
			if err := oauth.NewOAuth(lekko.NewBFFClient(rs)).ConfirmForgotPassword(cmd.Context(), email, newPassword, confirmNewPassword, verificationCode); err != nil {
				return err
			}
			fmt.Printf("Password reset successful!")
			return nil
		},
	}
	return cmd
}

func resendVerification() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resend-verification",
		Short: "Resend email verification",
		RunE: func(cmd *cobra.Command, args []string) error {
			var email string
			rs := secrets.NewSecretsOrFail()
			if err := survey.AskOne(&survey.Input{
				Message: "Email:",
			}, &email); err != nil {
				return errors.Wrap(err, "prompt email")
			}
			if err := oauth.NewOAuth(lekko.NewBFFClient(rs)).ResendVerification(cmd.Context(), email); err != nil {
				return err
			}
			fmt.Printf("An email with a verification code has been sent to %s\n", email)
			fmt.Println("Run `lekko auth confirm` to verify email.")
			return nil
		},
	}
	return cmd
}
