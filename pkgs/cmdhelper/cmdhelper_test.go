package cmdhelper

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

func TestCMDHelper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CMDHelper Suite")
}

var _ = Describe("Main", func() {
	var root *cobra.Command
	var ch *CmdHelper

	BeforeEach(func() {
		root = &cobra.Command{}
		ch = NewCmdHelper(root)
	})

	Describe(".addToGroup", func() {
		It("should add flag to group", func() {
			ch.addToGroup("grp1", "flag1")
			Expect(ch.group).To(HaveLen(1))
			Expect(ch.group[0].name).To(Equal("grp1"))
			Expect(ch.group[0].flags).To(HaveLen(1))
			Expect(ch.group[0].flags[0]).To(Equal("flag1"))
		})
	})

	Describe(".findGroup", func() {
		It("should return group by name", func() {
			ch.addToGroup("grp1", "flag1")
			g := ch.findGroup("grp1")
			Expect(g).ToNot(BeNil())
			Expect(g.name).To(Equal("grp1"))
			Expect(g.flags).To(HaveLen(1))
		})

		It("should return nil if group was not found", func() {
			ch.addToGroup("grp1", "flag1")
			g := ch.findGroup("grp2")
			Expect(g).To(BeNil())
		})
	})

	Describe(".getFlagGroup", func() {
		It("should get the group of a flag", func() {
			ch.addToGroup("grp1", "flag1")
			g := ch.getFlagGroup("flag1")
			Expect(g).To(Not(BeNil()))
			Expect(g.name).To(Equal("grp1"))
		})
	})

	Describe(".Grp", func() {
		It("should add flags to same group", func() {
			ch.Grp("grp1", "flag1")
			ch.Grp("grp1", "flag2")
			Expect(ch.group).To(HaveLen(1))
			Expect(ch.group[0].name).To(Equal("grp1"))
			Expect(ch.group[0].flags).To(HaveLen(2))
			Expect(ch.group[0].flags[0]).To(Equal("flag1"))
			Expect(ch.group[0].flags[1]).To(Equal("flag2"))
		})
	})
})
