package lekkomonkey

type OneArgs struct {
	One string
}

// One
func getOne(args *OneArgs) string {
	return "one"
}

type AnyName struct {
	Two string
}

// Two
func getTwo(args *AnyName) string {
	return "two"
}
