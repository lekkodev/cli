package lekkofrontend

// Default editor mode
func getDefaultEditorMode(username string) string {
	if username == "non-engineer@lekko.com" {
		return "visual"
	}
	return "code"
}
