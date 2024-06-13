package lekkomonkey

type OneArgs struct {
	One string
}

// One
func getOne(args *OneArgs) string {
	return "one"
}

type TwoArgs struct {
	Two string
}

// Two
func getTwo(args *TwoArgs) string {
	return "two"
}
