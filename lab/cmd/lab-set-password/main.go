package main

import (
	"fmt"
	"os"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: lab-set-password <panel.json> <password>")
		os.Exit(1)
	}
	store, err := config.NewStore(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := store.SetPassword("admin", os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
