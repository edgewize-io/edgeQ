package e2e_test

import (
	"fmt"
	. "github.com/edgewize/edgeQ/tests/e2e"
)

type Dog struct {
	Name       string
	Color      string
	Height     int
	Vaccinated bool
}

func ExampleMetricFromStruct() {

	var dogs = []Dog{
		{"Buster", "Black", 56, false},
		{"Buster", "Black", 56, false},
		{"Jake", "White", 61, false},
		{"Bingo", "Brown", 50, true},
		{"Gray", "Cream", 68, false},
	}

	m := MetricFromStruct(dogs)

	fmt.Println(m.GroupBy("Name", "Color"))

	// Output:
	//map[Bingo_Brown:1 Buster_Black:2 Gray_Cream:1 Jake_White:1]
}
