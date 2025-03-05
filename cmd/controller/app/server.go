package app

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/config"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"os"
	"os/signal"
	"syscall"
)

var (
	configuration config.ControllerConfig
)

func NewControllerCommand() *cobra.Command {
	// TODO: add description
	// TODO: add version flag
	// TODO: check Etcd Connectivity
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "The Controller ...",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := config.LoadConfiguration(&configuration)
			if err != nil {
				return errors.New("error: failed to load Controller configuration. " + err.Error())
			}
			// TODO: validate configuration parameters
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			logger.InitLogger(configuration.LogDebug, configuration.LogCaller, configuration.LogStacktrace)
			c, err := NewController()
			if err != nil {
				logger.Fatalf("Error creating Controller: %s", err.Error())
			}
			logger.Infof("Server running on %s", configuration.Port)
			Run(c)
		},
	}
	return cmd
}

func NewController() (*controller.Controller, error) {
	c, err := controller.NewController(configuration)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Run(c *controller.Controller) {
	logger.Infof("Version: ...")

	// Create a context that is canceled on SIGINT or SIGTERM signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func(EtcdClient *etcd.EtcdClient) {
		logger.Debugf("Closing etcd client")
		err := EtcdClient.Close()
		if err != nil {
			logger.Fatalf(err)
		}
	}(c.EtcdClient)

	go c.RunWithContext(ctx)
	go c.ListenAndServe(configuration)

	// Wait for context cancellation
	<-ctx.Done()
	logger.Infof("Shutting down")
}
