package options

import (
	"flag"
	"github.com/edgewize/modelmesh/internal/msc/config"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"strings"
)

type ServerRunOptions struct {
	*config.Config
}

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{
		Config: config.New(),
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
