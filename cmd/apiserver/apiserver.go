package main

import (
	"fmt"
	"os"

	"go.datum.net/iam/cmd/apiserver/app"
)

func main() {
	if err := app.Command().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
