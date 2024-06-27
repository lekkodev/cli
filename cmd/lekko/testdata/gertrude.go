package lekkogertrude

type Widget struct {
	Name string
	Size float64
}

func getWidget() *Widget {
	return &Widget{
		Name: "Terry",
		Size: 42.8,
	}
}
