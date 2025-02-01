/*
Copyright 2020 KubeSphere Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"flag"
	"github.com/edgewize/modelmesh/internal/broker"
	"github.com/edgewize/modelmesh/internal/broker/config"
	"k8s.io/klog/v2"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
)

type ServerRunOptions struct {
	ConfigFile string
	*config.Config

	DebugMode bool
}

func NewServerRunOptions() *ServerRunOptions {
	s := &ServerRunOptions{
		Config: config.New(),
	}

	return s
}

func (s *ServerRunOptions) Flags() (fss cliflag.NamedFlagSets) {
	fs := fss.FlagSet("generic")
	fs.BoolVar(&s.DebugMode, "debug", false, "Don't enable this if you don't know what it means.")

	fs = fss.FlagSet("klog")
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = strings.Replace(fl.Name, "_", "-", -1)
		fs.AddGoFlag(fl)
	})

	return fss
}

// const fakeInterface string = "FAKE"

// NewServer creates an BrokerServer instance using given options
func (s *ServerRunOptions) NewServer(stopCh <-chan struct{}) (*broker.Server, error) {
	brokerServer := &broker.Server{
		Config: s.Config,
	}
	return brokerServer, nil
}
