package main

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/llmariner/cluster-manager/pkg/status"
	"github.com/llmariner/cluster-monitor/agent/internal/collector"
	"github.com/llmariner/cluster-monitor/agent/internal/config"
	"github.com/llmariner/cluster-monitor/agent/internal/sender"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/rbac-manager/pkg/auth"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func runCmd() *cobra.Command {
	var path string
	var logLevel int
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Parse(path)
			if err != nil {
				return err
			}
			if err := c.Validate(); err != nil {
				return err
			}
			stdr.SetVerbosity(logLevel)
			if err := run(cmd.Context(), &c); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "config", "", "Path to the config file")
	cmd.Flags().IntVar(&logLevel, "v", 0, "Log level")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func run(ctx context.Context, c *config.Config) error {
	logger := stdr.New(log.Default())
	log := logger.WithName("boot")
	ctx = ctrl.LoggerInto(ctx, log)
	ctrl.SetLogger(logger)

	if err := auth.ValidateClusterRegistrationKey(); err != nil {
		return err
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		LeaderElection:   c.KubernetesManager.EnableLeaderElection,
		LeaderElectionID: c.KubernetesManager.LeaderElectionID,
		Metrics: metricsserver.Options{
			BindAddress: c.KubernetesManager.MetricsBindAddress,
		},
		HealthProbeBindAddress: c.KubernetesManager.HealthBindAddress,
		PprofBindAddress:       c.KubernetesManager.PprofBindAddress,
	})
	if err != nil {
		return err
	}

	if c.ComponentStatusSender.Enable {
		ss, err := status.NewBeaconSender(c.ComponentStatusSender, grpcOption(c), logger)
		if err != nil {
			return err
		}
		go func() {
			ss.Run(logr.NewContext(ctx, logger))
		}()
	}

	col := collector.New(c.GPUTelemetry.PrometheusURL)
	if err := col.SetupWithManager(mgr); err != nil {
		return err
	}

	conn, err := grpc.NewClient(c.ClusterMonitorServerWorkerServiceAddr, grpcOption(c))
	if err != nil {
		return err
	}
	telemetryClient := v1.NewClusterMonitorWorkerServiceClient(conn)

	se := sender.New(telemetryClient, col.PayloadCh())
	if err := se.SetupWithManager(mgr); err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	return mgr.Start(ctx)

}

func grpcOption(c *config.Config) grpc.DialOption {
	if c.Worker.TLS.Enable {
		return grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	}
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}
