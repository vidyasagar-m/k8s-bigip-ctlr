package cccl

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestAS3(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CCCL Suite")
}
