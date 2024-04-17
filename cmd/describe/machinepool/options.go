package machinepool

import "github.com/openshift/rosa/pkg/reporter"

type DescribeMachinepoolUserOptions struct {
	machinepool string
}

type DescribeMachinepoolOptions struct {
	reporter *reporter.Object

	args DescribeMachinepoolUserOptions
}

func NewDescribeMachinepoolUserOptions() DescribeMachinepoolUserOptions {
	return DescribeMachinepoolUserOptions{machinepool: ""}
}

func NewDescribeMachinepoolOptions() (*DescribeMachinepoolOptions, error) {
	return &DescribeMachinepoolOptions{
		reporter: reporter.CreateReporter(),
		args:     DescribeMachinepoolUserOptions{},
	}, nil
}

func (m *DescribeMachinepoolOptions) Machinepool() string {
	return m.args.machinepool
}

func (m *DescribeMachinepoolOptions) Bind(args DescribeMachinepoolUserOptions) {
	m.args.machinepool = args.machinepool
}
