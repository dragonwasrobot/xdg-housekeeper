package xdg

import (
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

func TestXdg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "xdg")
}
