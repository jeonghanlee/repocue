package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/jeonghanlee/repocue/internal/codexadapter"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if os.Getenv("REPOCUE_EVAL_SCHEMA") == taskbenchmark.RunnerVersion {
		config, err := codexadapter.TaskConfigFromEnvironment()
		if err == nil {
			observation, runErr := codexadapter.RunTask(ctx, config)
			err = json.NewEncoder(os.Stdout).Encode(observation)
			if err == nil {
				err = runErr
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
