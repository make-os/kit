package types

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepoProposals", func() {
	Describe(".Add", func() {
		It("should add new proposal successfully", func() {
			rp := RepoProposals(map[string]interface{}{})
			rp.Add("1", &RepoProposal{Creator: "address1"})
			rp.Add("2", &RepoProposal{Creator: "address1"})
			Expect(rp).To(HaveLen(2))
		})
	})

	Describe(".Has", func() {
		It("should return true if proposal with target ID exist", func() {
			rp := RepoProposals(map[string]interface{}{})
			rp.Add("1", &RepoProposal{Creator: "address1"})
			Expect(rp.Has("2")).To(BeFalse())
			Expect(rp.Has("1")).To(BeTrue())
		})
	})

	Describe(".Get", func() {
		It("should return the expected value when proposal with target ID exist", func() {
			rp := RepoProposals(map[string]interface{}{})
			rp.Add("1", &RepoProposal{Creator: "address1"})
			res := rp.Get("1")
			Expect(res).ToNot(BeNil())
			Expect(res.Creator).To(Equal("address1"))
		})

		It("should return nil when proposal with target ID does not exist", func() {
			rp := RepoProposals(map[string]interface{}{})
			res := rp.Get("1")
			Expect(res).To(BeNil())
		})
	})

	Describe(".ForEach", func() {
		It("should loop through all items as long as iteratee does not return error", func() {
			rp := RepoProposals(map[string]interface{}{})
			rp.Add("1", &RepoProposal{Creator: "address1"})
			rp.Add("2", &RepoProposal{Creator: "address1"})
			var idsIterated = map[string]struct{}{}
			rp.ForEach(func(prop *RepoProposal, id string) error {
				idsIterated[id] = struct{}{}
				return nil
			})
			Expect(idsIterated).To(HaveLen(2))
		})

		It("should stop iterating through items once iteratee returns error", func() {
			rp := RepoProposals(map[string]interface{}{})
			rp.Add("1", &RepoProposal{Creator: "address1"})
			rp.Add("2", &RepoProposal{Creator: "address1"})
			var idsIterated = map[string]struct{}{}
			expectedErr := fmt.Errorf("bad thing happened")
			err := rp.ForEach(func(prop *RepoProposal, id string) error {
				idsIterated[id] = struct{}{}
				return expectedErr
			})
			Expect(idsIterated).To(HaveLen(1))
			Expect(err).To(Equal(expectedErr))
		})
	})
})
