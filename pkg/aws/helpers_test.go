package aws_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/rosa/pkg/aws"
)

var _ = Describe("Helpers", func() {
	DescribeTable("UserTagValidator tests",
		func(input string, expectedErrorSubstring string) {
			err := aws.UserTagValidator(input)
			if expectedErrorSubstring == "" {
				Expect(err).ToNot(HaveOccurred(), "Error should not have occurred")
			} else {
				Expect(err).To(HaveOccurred(),
					"An error should have occurred")
				Expect(err.Error()).To(ContainSubstring(expectedErrorSubstring),
					"The error should have contained substring")
			}
		},
		Entry("No error for valid tag",
			"foo:bar", ""),
		Entry("No error for valid tags",
			"foo:bar,bar:baz", ""),
		Entry("Tag is missing colon seperator",
			"foobar", "invalid tag format"),
		Entry("Tag key exceeds regex length requirements",
			strings.Repeat("k", 129)+":v", "expected a valid user tag key"),
		Entry("Tag key does not meet minimum regex length requirements",
			":v", "expected a valid user tag key"),
		Entry("Tag value exceeds regex length requirements",
			"k:"+strings.Repeat("v", 257), "expected a valid user tag value"),
	)

})
