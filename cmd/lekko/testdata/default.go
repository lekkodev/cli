package lekkodefault

import (
	"strings"

	"golang.org/x/exp/slices"
)

type DBConfig struct {
	MaxIdleConns int64
	MaxOpenConns int64
}

type ErrorFilterConfig struct {
	LogLevel int64
}

type MemcachedConfig struct {
	MaxIdleConns int64
	TimeoutMs    int64
}

type OAuthDeviceConfig struct {
	VerificationUri        string
	PollingIntervalSeconds int64
}

type RegistrationConfig struct {
	RegistrationBaseUrl    string
	TokenExpirationMinutes int64
	EmailTemplate          string
	EmailSubject           string
}

type RolloutConfiguration struct {
	LockTtl           int64
	NumRolloutWorkers int64
	RolloutTimeout    int64
	Delay             int64
	Jitter            int64
	ChanBufferSize    int64
}

type RolloutContentsConfig struct {
	Compare        bool
	UseGitContents bool
}

type TeamExemptProcedures struct {
	Procedures []string
}

// Whether or not the backend returns the statically parsed feature for a feature of type Proto..
func getBffShowStaticProto(env string, feature string, owner string, repo string) bool {
	if repo == "staging-types" && feature == "proto_test" {
		return true
	} else if repo == "proto-test" {
		return true
	} else if env == "staging" {
		return true
	} else if owner == "david-lekko-test" {
		return true
	} else if owner == "lekko-coco-demo" {
		return true
	} else if owner == "lekkodev" {
		return true
	}
	return false
}

// Teams that can access the Lekko Docs.
func getCanAccessDocs(teamname string) bool {
	if slices.Contains([]string{"lekko", "lekkodev"}, teamname) {
		return true
	} else if teamname == "trunk" {
		return true
	} else if teamname == "champify" {
		return true
	} else if teamname == "stytch" {
		return true
	} else if teamname == "roserocket" {
		return true
	} else if teamname == "atob" {
		return true
	} else if teamname == "robust-intelligence" {
		return true
	} else if slices.Contains([]string{"bouyant", "buoyant"}, teamname) {
		return true
	} else if teamname == "zip" {
		return true
	} else if teamname == "buf" {
		return true
	} else if teamname == "wellen.ai" {
		return true
	} else if teamname == "mnemonic" {
		return true
	} else if teamname == "pocus" {
		return true
	} else if teamname == "aurora" {
		return true
	} else if teamname == "ecl" {
		return true
	} else if teamname == "pearson" {
		return true
	} else if teamname == "bigeye" {
		return true
	} else if teamname == "portalhq" {
		return true
	} else if teamname == "ramp" {
		return true
	} else if teamname == "temporal" {
		return true
	} else if teamname == "cointracker" {
		return true
	} else if teamname == "socket" {
		return true
	} else if teamname == "tecton" {
		return true
	} else if teamname == "develophealth" {
		return true
	} else if teamname == "helicone" {
		return true
	} else if teamname == "coco" {
		return true
	}
	return false
}

// Number of days to look back when querying for context key usage metrics
func getContextKeyMetricsLookbackDays() int64 {
	return 3
}

// How many seconds after creation does a Lekko browser cookie expire
func getCookieExpirationDuration() int64 {
	return 86400
}

// Config options for backend DB client
func getDbConfig() *DBConfig {
	return &DBConfig{MaxIdleConns: 64}
}

// Configuration for lekko's OAuth 2.0 device authorization process
func getDeviceOauth(env string) *OAuthDeviceConfig {
	if env == "staging" {
		return &OAuthDeviceConfig{
			PollingIntervalSeconds: 5,
			VerificationUri:        "https://app-staging.lekko.com/login/device",
		}
	} else if env == "development" {
		return &OAuthDeviceConfig{
			PollingIntervalSeconds: 5,
			VerificationUri:        "http://localhost:8080/login/device",
		}
	}
	return &OAuthDeviceConfig{
		PollingIntervalSeconds: 5,
		VerificationUri:        "https://app.lekko.com/login/device",
	}
}

// Whether configs should be filtered based on API key scopes on the read path
func getEnableApiKeyScoping() bool {
	return true
}

func getEnableMembershipsByDomainName(env string, username string) bool {
	if env == "development" {
		return true
	} else if strings.HasSuffix(username, "@lekko.com") {
		return true
	}
	return false
}

// Dynamically change error logs' levels. The context key "message" refers to the "error" field while "log" refers to "msg".
func getErrorFilterConfig(grpcErrorCode float64, log string, message string) *ErrorFilterConfig {
	if slices.Contains([]float64{3, 5, 6, 7, 11, 16}, grpcErrorCode) {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "unsupported static parsing") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "Found feature(s) with compilation or formatting diffs") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "deleting invalid in-mem repo") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "context canceled") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "acquire lock") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(log, "unable to get reset_at cache value") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(log, "unable to reset quota") {
		return &ErrorFilterConfig{LogLevel: 3}
	} else if strings.Contains(message, "field contextProto not found in type metadata.NamespaceConfigRepoMetadata") {
		return &ErrorFilterConfig{LogLevel: 3}
	}
	return &ErrorFilterConfig{LogLevel: 1}
}

// example bool config pushed from backend
func getExample(contextKey string) bool {
	if contextKey == "43" {
		return true
	}
	return true
}

// whether or not to force the background workers in the rollout handler to roll out any particular repo
func getForceRollout(env string) bool {
	if env == "production" {
		return true
	}
	return false
}

// Whether to use SHA512 as the hashing algorithm for generated API keys
func getHashApiKeySha() bool {
	return true
}

func getInviteMagicLinkUrl(env string) string {
	if env == "staging" {
		return "https://app-staging.lekko.com/authenticate"
	} else if env == "development" {
		return "http://localhost:5173/authenticate"
	}
	return "https://app.lekko.com/authenticate"
}

// whether or not we should avoid checking a user's GitHub auth for a particular rpc
func getIsGhauthExemptRpc(rpc string) bool {
	if slices.Contains([]string{"CreateRepository", "DeleteRepository", "GetUserGitHubRepos", "GetUserGitHubInstallations"}, rpc) {
		return false
	}
	return true
}

// my feature description
func getIsLekkoAdmin(username string) bool {
	if slices.Contains([]string{"dan@lekko.com", "shubhit@lekko.com", "konrad@lekko.com", "konradjniemiec@gmail.com", "shubhitms@gmail.com", "danielk@lekko.com", "david@lekko.com", "sergey@lekko.com"}, username) {
		return true
	}
	return false
}

// Config options for backend Memcached clients
func getMemcachedConfig(env string) *MemcachedConfig {
	if env == "development" {
		return &MemcachedConfig{
			MaxIdleConns: 128,
			TimeoutMs:    1000,
		}
	}
	return &MemcachedConfig{
		MaxIdleConns: 128,
		TimeoutMs:    200,
	}
}

// my feature description
func getMetricsBatchSize() int64 {
	return 2000
}

func getNewButtonThree(env string) string {
	if env == "production" {
		return "prod"
	}
	return "resolved conflict using fix pr"
}

func getNewFeatureFlag(env string) bool {
	if env == "development" {
		return true
	}
	return false
}

// Whether to only return the default branch info in the ListBranches endpoint. Switching branches and viewing PRs is disabled on the web UI for now, so this should be a quick temp fix for GitHub rate limit issues.
func getOnlyListDefaultBranch() bool {
	return true
}

// Settings for Lekko registration flow
func getRegistrationConfig(env string, userExists bool) *RegistrationConfig {
	if userExists && env == "development" {
		return &RegistrationConfig{
			EmailSubject: "[Dev] Your Lekko Login Link",
			EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Login Link</title>
</head>
<body>
	<p>Welcome back!</p>
	<p>Please click on the link below to log in. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Login</a>

	<p>If you did not request a log in link from Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
			RegistrationBaseUrl:    "http://localhost:5173/login",
			TokenExpirationMinutes: 60,
		}
	} else if userExists && env == "staging" {
		return &RegistrationConfig{
			EmailSubject: "[Staging] Your Lekko Login Link",
			EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Login Link</title>
</head>
<body>
	<p>Welcome back!</p>
	<p>Please click on the link below to log in. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Login</a>

	<p>If you did not request a log in link from Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
			RegistrationBaseUrl:    "https://app-staging.lekko.com/login",
			TokenExpirationMinutes: 60,
		}
	} else if userExists {
		return &RegistrationConfig{
			EmailSubject: "Your Lekko Login Link",
			EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Login Link</title>
</head>
<body>
	<p>Welcome back!</p>
	<p>Please click on the link below to log in. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Login</a>

	<p>If you did not request a log in link from Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
			RegistrationBaseUrl:    "https://app.lekko.com/login",
			TokenExpirationMinutes: 60,
		}
	} else if env == "staging" {
		return &RegistrationConfig{
			EmailSubject: "[Staging] Your Lekko Registration Link",
			EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Registration Link</title>
</head>
<body>
	<p>Welcome to Lekko!</p>
	<p>We're thrilled to have you join our community. Please click on your personal sign-up link below to complete your onboarding. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Sign Up</a>

	<p>After finishing your onboarding, you will be able to access all the features and services available on our platform.</p>

	<p>If you did not sign up for Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
			RegistrationBaseUrl:    "https://app-staging.lekko.com/signup",
			TokenExpirationMinutes: 60,
		}
	} else if env == "development" {
		return &RegistrationConfig{
			EmailSubject: "[Dev] Your Lekko Registration Link",
			EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Registration Link</title>
</head>
<body>
	<p>Welcome to Lekko!</p>
	<p>We're thrilled to have you join our community. Please click on your personal sign-up link below to complete your onboarding. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Sign Up</a>

	<p>After finishing your onboarding, you will be able to access all the features and services available on our platform.</p>

	<p>If you did not sign up for Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
			RegistrationBaseUrl:    "http://localhost:5173/signup",
			TokenExpirationMinutes: 60,
		}
	}
	return &RegistrationConfig{
		EmailSubject: "Your Lekko Registration Link",
		EmailTemplate: `<html>
<head>
	<meta charset="utf-8">
	<title>Your Lekko Registration Link</title>
</head>
<body>
	<p>Welcome to Lekko!</p>
	<p>We're thrilled to have you join our community. Please click on your personal sign-up link below to complete your onboarding. This link will expire in 1 hour.</p>

	<a href="{{.RegistrationURL}}" target="_blank" rel="noopener noreferrer">Sign Up</a>

	<p>After finishing your onboarding, you will be able to access all the features and services available on our platform.</p>

	<p>If you did not sign up for Lekko, please disregard this email.</p>

	<p>Best regards,</p>
	<p>Lekko Inc.</p>
</body>
</html>
`,
		RegistrationBaseUrl:    "https://app.lekko.com/signup",
		TokenExpirationMinutes: 60,
	}
}

func getReturnFdsToFe() bool {
	return true
}

// log level for rockset client, see https://github.com/rs/zerolog/blob/master/globals.go#L35-L48 for supported values
func getRocksetLoggerLevel(env string) string {
	if env == "staging" {
		return "debug"
	}
	return "warn"
}

// max elapsed time in seconds to retry rockset writes
func getRocksetRetryMaxElapsedTimeSecs() int64 {
	return 15
}

// Rollout handler configuration. All durations are in seconds.
func getRolloutConfiguration(env string) *RolloutConfiguration {
	if env == "staging" {
		return &RolloutConfiguration{
			ChanBufferSize:    100,
			Delay:             300,
			Jitter:            30,
			LockTtl:           60,
			NumRolloutWorkers: 250,
			RolloutTimeout:    120,
		}
	}
	return &RolloutConfiguration{
		ChanBufferSize:    100,
		Delay:             900,
		Jitter:            60,
		LockTtl:           300,
		NumRolloutWorkers: 250,
		RolloutTimeout:    480,
	}
}

// Controls how config repo contents are derived during rollout. Formerly a JSON config.
func getRolloutContentsProto(env string) *RolloutContentsConfig {
	if slices.Contains([]string{"staging", "development"}, env) {
		return &RolloutContentsConfig{UseGitContents: true}
	}
	return &RolloutContentsConfig{UseGitContents: true}
}

// Controls whether we sync with GitHub to fetch updated branch information after saving changes through BFF
func getSaveSyncBranchWithGithub() bool {
	return false
}

// my feature description
func getServiceRatelimits(env string, team string) int64 {
	if team == "test_team" || team == "lekkodev" || env == "staging" {
		return 1000000
	}
	return 5000
}

// If true, perform git operations using lekko bot
func getShouldMachineCommit() bool {
	return true
}

// whether or not bff service should validate that the team name in the request matches the team name in the cookie header.
func getShouldValidateTeam(procedure string) bool {
	if procedure == "ListAPIKeys" {
		return false
	} else if procedure == "GenerateAPIKey" {
		return false
	} else if procedure == "DeleteAPIKey" {
		return false
	} else if slices.Contains([]string{"CreateBranch", "Save", "Review", "Merge", "GetRepositoryContents"}, procedure) {
		return false
	}
	return true
}

// Configuration for BFF procedures that don't need to be checked for team info
func getTeamExemptProcedures() *TeamExemptProcedures {
	return &TeamExemptProcedures{
		Procedures: []string{
			"AuthorizeDevice",
			"ChangePassword",
			"CreateTeam",
			"DeleteUserOAuth",
			"GetUserGitHubInstallations",
			"GetUserLoggedInInfo",
			"GetUserOAuth",
			"ListUserMemberships",
			"OAuthUser",
			"UseTeam",
		},
	}
}

// Whether or not to rely on lekko's custom Any protobuf definition as source of truth..
func getUseCustomAny() bool {
	return true
}

// whether to use team name from request and ignore team name from cookie
func getUseTeamFromRequest(procedure string) bool {
	if slices.Contains([]string{"ListTeamMemberships"}, procedure) {
		return true
	}
	return false
}
