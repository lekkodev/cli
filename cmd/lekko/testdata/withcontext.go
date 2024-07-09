package lekkofrontend

type BlockedDomains struct {
	Domains []string
}

type DefaultEditorModeArgs struct {
	Username string
}

// Default editor mode
func getDefaultEditorMode(args *DefaultEditorModeArgs) string {
	if args.Username == "non-engineer@lekko.com" {
		return "visual"
	}
	return "code"
}

// Blocked auto-join domains
func getBlockedAutoJoinDomains() *BlockedDomains {
	return &BlockedDomains{
		Domains: []string{
			"gmail.com",
			"yahoo.com",
			"hotmail.com",
			"outlook.com",
			"aol.com",
			"icloud.com",
			"me.com",
		},
	}
}
