package lekkofrontend

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
