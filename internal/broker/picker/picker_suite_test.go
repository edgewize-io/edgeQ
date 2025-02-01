package picker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPicker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Picker Suite")
}
