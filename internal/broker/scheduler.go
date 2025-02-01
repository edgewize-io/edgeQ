package broker

import (
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	"k8s.io/klog/v2"
)

type SchedulerPick func() (picker.PickResult, error)

type Scheduler struct {
	builder   picker.PickerBuilder
	picker    picker.Picker
	errPicker picker.Picker
	pickInfo  picker.PickInfo
	reoucres  []*config.ServiceGroup
}

func NewScheduler(cfg *config.Schedule) (*Scheduler, error) {
	klog.Infof("cfg: %v", cfg.Method)
	builder := picker.Builder(cfg.Method)
	if builder == nil {
		return nil, fmt.Errorf("builder is nil")
	}

	errPicker := picker.NewErrPicker(fmt.Errorf("picker is nil"))
	return &Scheduler{builder: builder, errPicker: errPicker, picker: nil}, nil
}

func (s *Scheduler) RegeneratePicker(groups []*config.ServiceGroup) {
	pbi := picker.NewPickerBuildInfo()
	for _, group := range groups {
		pbi.AddResource(group)
	}
	s.picker = s.builder.Build(pbi)
}

func (s *Scheduler) Pick(info picker.PickInfo) (picker.PickResult, error) {
	if s.picker == nil {
		return s.errPicker.Pick(info)
	}
	return s.picker.Pick(info)
}
