package report

import (
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

func TestReport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "report")
}
