package options

import (
	"flag"
	"github.com/edgewize/edgeQ/internal/controller/config"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/leaderelection"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

type ServerRunOptions struct {
	*config.Config
	LeaderElect            bool
	LeaderElection         *leaderelection.LeaderElectionConfig
	MetricsBindAddress     string
	HealthProbeBindAddress string
}

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{
		Config: config.New(),
		LeaderElection: &leaderelection.LeaderElectionConfig{
			LeaseDuration: 30 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
		},
		LeaderElect:            false,
		MetricsBindAddress:     "0.0.0.0:18080",
		HealthProbeBindAddress: "0.0.0.0:18081",
	}
}

func (s *ServerRunOptions) Merge(conf *config.Config) {
	if conf == nil {
		return
	}

	if conf.KubernetesOptions != nil {
		s.KubernetesOptions = conf.KubernetesOptions
	}

	if conf.WebhookServiceName != "" {
		s.WebhookServiceName = conf.WebhookServiceName
	}

	if conf.ProxySidecar != nil {
		s.ProxySidecar = conf.ProxySidecar
	}

	if conf.BrokerSidecar != nil {
		s.BrokerSidecar = conf.BrokerSidecar
	}

	if conf.WebhookPort != 0 {
		s.WebhookPort = conf.WebhookPort
	}

}

func (s *ServerRunOptions) bindLeaderElectionFlags(l *leaderelection.LeaderElectionConfig, fs *pflag.FlagSet) {
	fs.DurationVar(&l.LeaseDuration, "leader-elect-lease-duration", l.LeaseDuration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	fs.DurationVar(&l.RenewDeadline, "leader-elect-renew-deadline", l.RenewDeadline, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	fs.DurationVar(&l.RetryPeriod, "leader-elect-retry-period", l.RetryPeriod, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
}

func (s *ServerRunOptions) Flags() (fss cliflag.NamedFlagSets) {
	fss = cliflag.NamedFlagSets{}
	s.KubernetesOptions.AddFlags(fss.FlagSet("kubernetes"), s.KubernetesOptions)

	fs := fss.FlagSet("leaderelection")
	s.bindLeaderElectionFlags(s.LeaderElection, fs)

	fs.BoolVar(&s.LeaderElect, "leader-elect", s.LeaderElect, ""+
		"Whether to enable leader election. This field should be enabled when controller manager"+
		"deployed with multiple replicas.")

	kfs := fss.FlagSet("klog")
	local := flag.NewFlagSet("klog", flag.ExitOnError)

	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = strings.Replace(fl.Name, "_", "-", -1)
		kfs.AddGoFlag(fl)
	})

	return
}
