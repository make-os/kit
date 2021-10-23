package types

import (
	"reflect"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxDetail", func() {
	Describe("Encode and Decode", func() {
		It("should encode and decode correctly", func() {
			txd := &TxDetail{
				RepoName:        "repo1",
				RepoNamespace:   "namespace",
				Reference:       "refs/heads/master",
				Fee:             "10.2",
				Value:           "12.3",
				Nonce:           1,
				PushKeyID:       "pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
				Signature:       "sig1",
				MergeProposalID: "1000",
				ReferenceData: &ReferenceData{
					IssueFields: &IssueFields{
						Labels:    []string{"lbl1", "lbl2"},
						Assignees: []string{"ass1", "ass2"},
					},
					MergeRequestFields: &MergeRequestFields{
						BaseBranch:       "base",
						BaseBranchHash:   "baseHash",
						TargetBranch:     "target",
						TargetBranchHash: "targetHash",
					},
					Close: pointer.ToBool(true),
				},
			}
			bz := txd.Bytes()
			Expect(txd.Bytes()).ToNot(BeEmpty())
			var txd2 TxDetail
			Expect(util.ToObject(bz, &txd2)).To(BeNil())
			Expect(reflect.DeepEqual(txd.ToMap(), txd2.ToMap())).To(BeTrue())
		})
	})
})

var _ = Describe("ReferenceTxDetails", func() {
	Describe("Get", func() {
		It("should get a reference tx details", func() {
			rtd := ReferenceTxDetails(map[string]*TxDetail{
				"refs/heads/dev": {
					RepoName:        "repo1",
					RepoNamespace:   "namespace",
					Reference:       "refs/heads/master",
					Fee:             "10.2",
					Value:           "12.3",
					Nonce:           1,
					PushKeyID:       "pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					Signature:       "sig1",
					MergeProposalID: "1000",
					ReferenceData: &ReferenceData{
						IssueFields: &IssueFields{
							Labels:    []string{"lbl1", "lbl2"},
							Assignees: []string{"ass1", "ass2"},
						},
						MergeRequestFields: &MergeRequestFields{
							BaseBranch:       "base",
							BaseBranchHash:   "baseHash",
							TargetBranch:     "target",
							TargetBranchHash: "targetHash",
						},
						Close: pointer.ToBool(true),
					},
				},
			})

			v := rtd.Get("refs/heads/dev")
			Expect(v).To(Equal(rtd["refs/heads/dev"]))
		})
	})
})

var _ = Describe("ReferenceData", func() {
	Describe("Get", func() {
		It("should get a reference tx details", func() {
			rd := &ReferenceData{
				IssueFields: &IssueFields{
					Labels:    []string{"lbl1", "lbl2"},
					Assignees: []string{"ass1", "ass2"},
				},
				MergeRequestFields: &MergeRequestFields{
					BaseBranch:       "base",
					BaseBranchHash:   "baseHash",
					TargetBranch:     "target",
					TargetBranchHash: "targetHash",
				},
				Close: pointer.ToBool(true),
			}

			bz := util.ToBytes(rd)
			var rd2 ReferenceData
			Expect(util.ToObject(bz, &rd2)).To(BeNil())
			Expect(reflect.DeepEqual(rd.ToMap(), rd2.ToMap())).To(BeTrue())
		})
	})
})
