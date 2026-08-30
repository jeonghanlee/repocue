package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/jeonghanlee/repocue/internal/codexadapter"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	config, err := codexadapter.ConfigFromEnvironment()
	if err == nil {
		var observation any
		observation, err = codexadapter.Run(ctx, config)
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(observation)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
