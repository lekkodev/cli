package lekkofrontend

type GetDefaultEditorModeArgs struct {
	Username string
}

// Default editor mode
func getDefaultEditorMode(args *GetDefaultEditorModeArgs) string {
	if args.Username == "non-engineer@lekko.com" {
		return "visual"
	}
	return "code"
}
