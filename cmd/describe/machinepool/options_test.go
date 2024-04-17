package machinepool

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test describe machinepool options", func() {
	var args DescribeMachinepoolUserOptions
	Context("Describe Machinepool User Options", func() {
		It("Creates default options", func() {
			args = NewDescribeMachinepoolUserOptions()
			Expect(args.machinepool).To(Equal(""))
		})
	})
	Context("Describe Machinepool Options", func() {
		var options DescribeMachinepoolOptions
		It("Create args from options using Bind (also tests MachinePool func)", func() {
			options.Bind(args)
			Expect(options.Machinepool()).To(Equal(""))
			// Set value then re-bind
			testMachinepool := "test"
			args.machinepool = testMachinepool
			options.Bind(args)
			Expect(options.Machinepool()).To(Equal(testMachinepool))
		})
	})
})
