// Command intania-shop-api starts and maintains the Intania Shop API.
package main

import (
	"fmt"
	"os"

	"github.com/esc-chula/intania-shop-api/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
