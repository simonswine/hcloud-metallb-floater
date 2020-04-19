package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var rootFlags = struct {
	MetricsAddr          string
	EnableLeaderElection bool
	Verbose              bool
	SyncPeriod           time.Duration
}{}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&rootFlags.Verbose, "verbose", "v", false, "Enable verbose logging")
	RootCmd.PersistentFlags().StringVar(&rootFlags.MetricsAddr, "metrics-addr", ":8585", "The address the metrics endpoint binds to.")
	RootCmd.PersistentFlags().BoolVar(&rootFlags.EnableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	RootCmd.PersistentFlags().DurationVar(&rootFlags.SyncPeriod, "sync-period", 5*time.Minute, "Sync peroid of the controller.")
}

func getEnvRequired(key string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		return value, nil
	}
	return "", fmt.Errorf("required environment variable missing: %s", key)
}

var appName = "hcloud-metallb-floater"

var RootCmd = &cobra.Command{
	Use: appName,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(args); err != nil {
			log.Fatal(err)
		}
	},
}
