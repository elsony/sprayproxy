/*
Copyright © 2023 The Spray Proxy Contributors

SPDX-License-Identifier: Apache-2.0
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/redhat-appstudio/sprayproxy/pkg/metrics"
	"github.com/redhat-appstudio/sprayproxy/pkg/server"
)

var (
	backends []string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the spray reverse proxy server",
	Long: `Run a reverse proxy that blindly "sprays" requests to one or more backend servers.
Use the --backend flag to specify which servers to forward traffic to:

sprayproxy server --backend http://localhost:8081 --backend http://localhost:8082
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		viper.AutomaticEnv()
		host := viper.GetString("host")
		port := viper.GetInt("port")
		metricsPort := viper.GetInt("metrics-port")
		// backends := viper.GetStringSlice("backend")
		insecureSkipTLSVerify := viper.GetBool("insecure-skip-tls-verify")
		crtFile := viper.GetString("metrics-cert")
		keyFile := viper.GetString("metrics-key")
		server, err := server.NewServer(host, port, insecureSkipTLSVerify, backends...)
		if err != nil {
			return err
		}

		metrics.InitMetrics(nil)
		stopCh := setupSignalHandler()
		metricsSrvr, err := metrics.NewServer(host, metricsPort, crtFile, keyFile)
		if err != nil {
			return err
		}
		go func() {
			metricsSrvr.RunServer(stopCh)
		}()
		err = server.Run()
		metricsSrvr.StopServer()
		return err
	},
}

var (
	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
	onlyOneSignalHandler = make(chan struct{})
)

func init() {
	rootCmd.AddCommand(serverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	viper.SetDefault("host", "")
	viper.SetDefault("port", 8080)
	viper.SetDefault("metrics-port", metrics.MetricsPort)
	viper.SetDefault("metrics-cert", "")
	viper.SetDefault("metrics-key", "")

	viper.SetEnvPrefix("SPRAYPROXY_SERVER")
	// Replace "-" with underscores "_"
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	serverCmd.Flags().String("host", "", "Host for running the server. Defaults to localhost")
	serverCmd.Flags().Int("port", 8080, "Port for running the server. Defaults to 8080")
	serverCmd.Flags().StringSliceVar(&backends, "backend", []string{}, "Backend to forward requests. Use more than once.")
	serverCmd.Flags().Bool("insecure-skip-tls-verify", false, "Skip TLS verification on all backends. INSECURE - do not use in production.")
	serverCmd.Flags().Int("metrics-port", metrics.MetricsPort, fmt.Sprintf("Port for the prometheus metrics endpoint.  Defaults to %d", metrics.MetricsPort))
	serverCmd.Flags().String("metrics-cert", "", "TLS Certificate file for the prometheus metric endpoint.  Defaults to empty, meaning TLS will not be used")
	serverCmd.Flags().String("metrics-key", "", "TLS Key file for the prometheus metric endpoint.  Defaults to empty, meaning TLS will not be used")

	viper.BindPFlags(serverCmd.Flags())

}

// setupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
