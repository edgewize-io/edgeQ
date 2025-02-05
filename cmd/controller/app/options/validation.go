package options

import "fmt"

func (s *ServerRunOptions) Validate() []error {
	var errors []error
	if s.Config.ProxySidecar == nil {
		errors = append(errors, fmt.Errorf("app injection config cannot be empty"))
	}

	if s.Config.BrokerSidecar == nil {
		errors = append(errors, fmt.Errorf("model injection config cannot be empty"))
	}

	return errors
}
