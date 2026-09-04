package plan

import (
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

func TestPlan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "plan")
}
