package util

import "math/rand"

func GreenColor() [3]float32 {
	return [3]float32{0, 1, 0}
}

func RandomColor() [3]float32 {
	return [3]float32{rand.Float32(), rand.Float32(), rand.Float32()}
}

func BlueColor() [3]float32 {
	return [3]float32{0, 0, 1}
}

func RedColor() [3]float32 {
	return [3]float32{1, 0, 0}
}

func YellowColor() [3]float32 {
	return [3]float32{1, 1, 0}
}
