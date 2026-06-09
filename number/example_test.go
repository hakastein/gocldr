package number_test

import (
	"fmt"

	"github.com/hakastein/gocldr/number"
	_ "github.com/hakastein/gocldr/number/locales/all"
)

func ExampleFormat() {
	fmt.Println(number.Format("de", 1234.5, number.Options{}))
	// Output: 1.234,5
}

func ExampleFormat_currency() {
	fmt.Println(number.Format("en", 1234, number.Options{Style: "currency", Currency: "USD"}))
	// Output: $1,234.00
}
