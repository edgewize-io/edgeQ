package picker

import (
	"fmt"
	errs "github.com/edgewize/edgeQ/internal/broker/error"
	"github.com/mroth/weightedrand/v2"
)

type WRRPickerBuilder struct{}

func (pb *WRRPickerBuilder) Build(info *PickerBuildInfo) Picker {
	if len(info.Resources) == 0 {
		return NewErrPicker(errs.ErrNoSubConnAvailable)
	}

	weights := map[string]int32{}
	for sc, scInfo := range info.Resources {
		if scInfo.Attributes != nil {
			val := scInfo.Attributes.Value(WeightAttributeKey)
			weight := val.(int32)
			weights[sc.ResourceName()] = weight
		}
	}

	return &wrrPicker{weights: weights}
}

type wrrPicker struct {
	weights map[string]int32
}

func (p *wrrPicker) Pick(opts PickInfo) (result PickResult, err error) {
	result = PickResult{}

	if len(opts.Candidates) <= 0 {
		err = errs.ErrNoSubConnAvailable
		return
	}

	choices := []weightedrand.Choice[Resource, int32]{}
	for _, candidate := range opts.Candidates {
		respWeight, ok := p.weights[candidate.ResourceName()]
		if !ok {
			err = fmt.Errorf("service group %s not register", candidate.ResourceName())
			return
		}
		choices = append(choices, weightedrand.NewChoice(candidate, respWeight))
	}

	c, err := weightedrand.NewChooser(choices...)
	if err != nil {
		return
	}

	result = PickResult{Resource: c.Pick()}
	return
}
