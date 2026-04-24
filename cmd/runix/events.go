package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/runixio/runix/internal/events"
	"github.com/spf13/cobra"
)

func newEventsCmd() *cobra.Command {
	var (
		follow    bool
		sinceStr  string
		eventType string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "View process events",
		Long:  `View event history for managed processes. Use --follow to stream events in real time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dd := dataDir()
			bus := events.NewBus(dd)
			defer bus.Close()

			since := time.Time{}
			if sinceStr != "" {
				d, err := time.ParseDuration(sinceStr)
				if err != nil {
					return fmt.Errorf("invalid duration %q: %w", sinceStr, err)
				}
				since = time.Now().Add(-d)
			}

			var filterTypes []events.EventType
			if eventType != "" {
				filterTypes = append(filterTypes, events.EventType(eventType))
			}

			// Show historical events.
			history := bus.History(since, filterTypes...)
			for _, evt := range history {
				printEvent(evt)
			}

			if !follow {
				return nil
			}

			// Follow: subscribe to new events.
			var ch <-chan events.Event
			if len(filterTypes) > 0 {
				ch = bus.Subscribe(filterTypes...)
			} else {
				ch = bus.Subscribe()
			}
			defer bus.Unsubscribe(ch)

			for evt := range ch {
				printEvent(evt)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow event stream")
	cmd.Flags().StringVar(&sinceStr, "since", "1h", "show events since duration (e.g. 10m, 1h, 24h)")
	cmd.Flags().StringVarP(&eventType, "type", "t", "", "filter by event type (e.g. process.crashed)")

	return cmd
}

func printEvent(evt events.Event) {
	data, _ := json.Marshal(evt)
	fmt.Fprintln(os.Stdout, string(data))
}
