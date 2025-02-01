/*
Copyright 2019 The KubeSphere Authors.

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

package app

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/cmd/broker/app/options"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func NewAPIServerCommand() (cmd *cobra.Command) {
	s := options.NewServerRunOptions()

	// Load configuration from file /etc/schedule/schedule.yaml
	conf, err := config.TryLoadFromDisk()
	if err == nil {
		s = &options.ServerRunOptions{
			Config: conf,
		}
	} else {
		klog.Fatal("Failed to load configuration from disk", err)
	}
	ret, _ := yaml.Marshal(conf)
	fmt.Println(string(ret))

	cmd = &cobra.Command{
		Use: "Schedule API Server",
		Long: `The Schedule API server validates and configures data for the API objects. 
The API Server services REST operations and provides the frontend to the
cluster's shared state through which all other components interact.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := s.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			return Run(s, config.WatchConfigChange(), signals.SetupSignalHandler())
		},
		SilenceUsage: true,
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, 0)
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of KubeSphere DevOps apiserver",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO implement the version output
		},
	}

	cmd.AddCommand(versionCmd)
	return
}

func Run(s *options.ServerRunOptions, configCh <-chan config.Config, ctx context.Context) error {
	ictx, cancelFunc := context.WithCancel(context.TODO())
	errCh := make(chan error)
	defer close(errCh)
	go func() {
		if err := run(s, ictx); err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case <-ctx.Done():
			cancelFunc()
			return nil
		case cfg := <-configCh:
			ret, _ := yaml.Marshal(cfg)
			fmt.Println(string(ret))
			cancelFunc()
			s.Config = &cfg
			ictx, cancelFunc = context.WithCancel(context.TODO())
			go func() {
				if err := run(s, ictx); err != nil {
					errCh <- err
				}
			}()
		case err := <-errCh:
			cancelFunc()
			return err
		}
	}

}

func run(s *options.ServerRunOptions, ctx context.Context) error {
	apiserver, err := s.NewServer(ctx.Done())
	if err != nil {
		return err
	}

	err = apiserver.PrepareRun(ctx.Done())
	if err != nil {
		klog.Errorf("Failed to prepare apiserver: %v", err)
		return err
	}

	return apiserver.Run(ctx)
}
