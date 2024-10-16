package lekkogertrude

type Widget struct {
	Name string
	Size float64
}

func getDirect() float64 {
	return getSize()
}

func getSize() float64 {
	return 43.1
}

func getWidget() *Widget {
	return &Widget{
		Name: "Terry",
		Size: getSize(),
	}
}
