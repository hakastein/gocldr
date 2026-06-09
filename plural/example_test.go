package plural_test

import (
	"fmt"

	"github.com/hakastein/gocldr/plural"
)

func ExampleCardinalFor() {
	// Russian: 2 is "few" (cardinal category).
	fmt.Println(plural.CardinalFor("ru", 2, 0, 0))
	// Output: few
}
