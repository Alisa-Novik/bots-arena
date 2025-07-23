package util

import "math/rand"

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

func GreenColor() [3]float32     { return [3]float32{0, 1, 0} }
func CyanColor() [3]float32      { return [3]float32{0, 1, 1} }
func MagentaColor() [3]float32   { return [3]float32{1, 0, 1} }
func WhiteColor() [3]float32     { return [3]float32{1, 1, 1} }
func BlackColor() [3]float32     { return [3]float32{0, 0, 0} }
func OrangeColor() [3]float32    { return [3]float32{1, 0.5, 0} }
func PurpleColor() [3]float32    { return [3]float32{0.5, 0, 0.5} }
func PinkColor() [3]float32      { return [3]float32{1, 0.75, 0.8} }
func LightBlueColor() [3]float32 { return [3]float32{0.53, 0.81, 0.92} }
func GreyColor() [3]float32      { return [3]float32{0.5, 0.5, 0.5} }
