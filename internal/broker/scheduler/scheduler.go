package scheduler

import (
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	"github.com/edgewize/edgeQ/pkg/constants"
	"k8s.io/klog/v2"
)

type Scheduler struct {
	builder   picker.PickerBuilder
	picker    picker.Picker
	errPicker picker.Picker
}

func NewScheduler(cfg config.Schedule) (*Scheduler, error) {
	klog.Infof("cfg: %v", cfg.Method)
	builder := picker.Builder(cfg.Method)
	if builder == nil {
		return nil, fmt.Errorf("builder is nil")
	}

	errPicker := picker.NewErrPicker(fmt.Errorf("picker is nil"))
	return &Scheduler{builder: builder, errPicker: errPicker, picker: nil}, nil
}

func (s *Scheduler) RegeneratePicker(groups []config.ServiceGroup) {
	pbi := picker.NewPickerBuildInfo()
	defaultExit := false
	for _, group := range groups {
		if group.ResourceName() == constants.DefaultServiceGroup {
			defaultExit = true
		}

		pbi.AddResource(group)
	}

	if !defaultExit {
		defaultSG := config.ServiceGroup{Name: constants.DefaultServiceGroup, Weight: 1}
		pbi.AddResource(defaultSG)
	}

	s.picker = s.builder.Build(pbi)
}

func (s *Scheduler) Pick(info picker.PickInfo) (picker.PickResult, error) {
	if s.picker == nil {
		return s.errPicker.Pick(info)
	}
	return s.picker.Pick(info)
}
