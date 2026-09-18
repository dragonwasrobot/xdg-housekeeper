package fsys

import (
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

func TestFsys(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "fsys")
}
