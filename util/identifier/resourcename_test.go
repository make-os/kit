package identifier_test

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/util/identifier"
)

var _ = Describe("Identifier", func() {
	Describe(".IsValidResourceName", func() {
		Specify("cases", func() {
			err := identifier.IsValidResourceName("abc&*")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid identifier; only alphanumeric, _, and - characters are allowed"))

			err = identifier.IsValidResourceName(strings.Repeat("a", 129))
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("name is too long. Maximum character length is 128"))

			err = identifier.IsValidResourceName("a")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("name is too short. Must be at least 3 characters long"))

			err = identifier.IsValidResourceName("abcdef_-")
			Expect(err).To(BeNil())

			err = identifier.IsValidResourceName("-abc")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid identifier; identifier cannot start with _ or - character"))

			err = identifier.IsValidResourceName("_abc")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid identifier; identifier cannot start with _ or - character"))
		})
	})

	Describe(".IsValidResourceNameNoMinLen", func() {
		It("cases", func() {
			Expect(identifier.IsValidResourceNameNoMinLen("myname")).To(BeNil())
			Expect(identifier.IsValidResourceNameNoMinLen("_myname")).ToNot(BeNil())
			Expect(identifier.IsValidResourceNameNoMinLen("a")).To(BeNil())
			Expect(identifier.IsValidResourceNameNoMinLen("a_b_c")).To(BeNil())
			Expect(identifier.IsValidResourceNameNoMinLen("_")).ToNot(BeNil())
			Expect(identifier.IsValidResourceNameNoMinLen("")).ToNot(BeNil())
		})
	})
})
