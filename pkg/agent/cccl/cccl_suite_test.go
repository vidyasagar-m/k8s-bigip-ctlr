package cccl

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestAS3(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CCCL Suite")
}
