package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

func TestXdgHousekeeper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "main")
}
