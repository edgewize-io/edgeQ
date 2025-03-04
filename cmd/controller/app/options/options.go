package options

import (
	"flag"
	"github.com/edgewize/edgeQ/internal/controller/config"
	"github.com/edgewize/edgeQ/pkg/simple/client/k8s"
	"k8s.io/client-go/tools/leaderelection"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

type ServerRunOptions struct {
	*config.Config
	KubernetesOptions      *k8s.KubernetesOptions
	LeaderElect            bool
	LeaderElection         *leaderelection.LeaderElectionConfig
	MetricsBindAddress     string
	HealthProbeBindAddress string
}

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{
		Config:            config.New(),
		KubernetesOptions: k8s.NewKubernetesOptions(),
		LeaderElection: &leaderelection.LeaderElectionConfig{
			LeaseDuration: 30 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
		},
		LeaderElect:            false,
		MetricsBindAddress:     ":18080",
		HealthProbeBindAddress: ":18081",
	}
}

func (s *ServerRunOptions) Flags() (fss cliflag.NamedFlagSets) {
	fs := fss.FlagSet("klog")
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = strings.Replace(fl.Name, "_", "-", -1)
		fs.AddGoFlag(fl)
	})

	return fss
}
