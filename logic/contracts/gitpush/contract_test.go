package gitpush_test

import (
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	logic2 "github.com/make-os/lobe/logic"
	"github.com/make-os/lobe/logic/contracts/gitpush"
	"github.com/make-os/lobe/logic/contracts/mergerequest"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/push/types"
	remotetypes "github.com/make-os/lobe/remote/types"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	crypto2 "github.com/make-os/lobe/util/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Contract", func() {
	var appDB, stateTreeDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var pushKeyID = crypto.CreatePushKeyID(crypto.StrToPublicKey("pushKeyID"))
	var pushKeyID2 = crypto.CreatePushKeyID(crypto.StrToPublicKey("pushKeyID2"))
	var rawPkID []byte

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
		rawPkID = crypto2.MustDecodePushKeyID(pushKeyID2)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CanExec", func() {
		It("should return true when able to execute tx type", func() {
			ct := gitpush.NewContract()
			Expect(ct.CanExec(txns.TxTypePush)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var repo = "repo1"
		var creator = crypto2.MustDecodePushKeyID(pushKeyID)
		var refs []*types.PushedReference
		var issueRef1 = plumbing.MakeIssueReference(1)
		var issue1RefData = &state.ReferenceData{Labels: []string{"label1", "label2"}, Assignees: []string{"key1", "key2"}}

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})
			logic.PushKeyKeeper().Update(pushKeyID, &state.PushKey{PubKey: crypto.StrToPublicKey("pub_key"), Address: sender.Addr()})
			logic.RepoKeeper().Update(repo, &state.Repository{
				Config: state.DefaultRepoConfig,
				References: map[string]*state.Reference{
					"refs/heads/master": {Nonce: 1, Creator: creator},
					issueRef1:           {Nonce: 1, Creator: creator, Data: issue1RefData},
				},
			})
		})

		When("pushed reference did not previously exist (new reference)", func() {
			It("should add pusher as creator of the reference", func() {
				refs = []*types.PushedReference{{Name: "refs/heads/dev", Data: &remotetypes.ReferenceData{}, Fee: "1"}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get("refs/heads/dev").Creator).To(Equal(crypto.PushKey(rawPkID)))
			})
		})

		When("pushed reference new hash is zero (meaning delete is required)", func() {
			It("should remove the reference from the repo", func() {
				refs = []*types.PushedReference{{Name: "refs/heads/master", NewHash: strings.Repeat("0", 40), Fee: "1"}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Has("refs/heads/master")).To(BeFalse())
			})
		})

		When("pushed reference already exist", func() {
			It("should not update reference creator", func() {
				refs = []*types.PushedReference{{Name: "refs/heads/master", Data: &remotetypes.ReferenceData{}, Fee: "1"}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get("refs/heads/master").Creator).ToNot(Equal(rawPkID))
				actual := crypto2.MustDecodePushKeyID(pushKeyID)
				Expect(rep.References.Get("refs/heads/master").Creator).To(Equal(crypto.PushKey(actual)))
			})
		})

		When("after a successful exec", func() {
			BeforeEach(func() {
				refs = []*types.PushedReference{{Name: "refs/heads/master", Data: &remotetypes.ReferenceData{}, Fee: "1"}}
				rawPkID := crypto2.MustDecodePushKeyID(pushKeyID)
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID, PusherAddress: sender.Addr()},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the reference's nonce was incremented", func() {
				repo := logic.RepoKeeper().Get(repo)
				Expect(repo.References.Get("refs/heads/master").Nonce.UInt64()).To(Equal(uint64(2)))
			})

			Specify("that (total pushed reference fee + total pushed reference secondary fees) were deducted from pusher account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})

			Specify("that sender account nonce was incremented", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(2)))
			})

			Specify("that the repository LastUpdated field is 1", func() {
				repo := logic.RepoKeeper().Get(repo)
				Expect(repo.LastUpdated.UInt64()).To(Equal(uint64(1)))
			})
		})

		When("pushed reference is an issue reference", func() {
			It("should add issue data from pushed reference", func() {
				ref := plumbing.MakeIssueReference(2)
				labels := []string{"lbl1"}
				assignees := []string{"key1"}
				cls := true
				refs = []*types.PushedReference{{
					Name: ref,
					Fee:  "1",
					Data: &remotetypes.ReferenceData{Close: &cls, IssueFields: remotetypes.IssueFields{Labels: &labels, Assignees: &assignees}},
				}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get(ref).Creator).To(Equal(crypto.PushKey(rawPkID)))
				Expect(rep.References[ref].Data.Closed).To(BeTrue())
				Expect(rep.References[ref].Data.Labels).To(Equal(labels))
				Expect(rep.References[ref].Data.Assignees).To(Equal(assignees))
			})

			It("should not alter issue data fields not specified in pushed reference", func() {
				cls := true
				refs = []*types.PushedReference{{
					Name: issueRef1,
					Fee:  "1",
					Data: &remotetypes.ReferenceData{Close: &cls}}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: creator},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get(issueRef1).Creator).To(Equal(crypto.PushKey(creator)))
				Expect(rep.References[issueRef1].Data.Closed).To(BeTrue())
				Expect(rep.References[issueRef1].Data.Labels).To(Equal(issue1RefData.Labels))
				Expect(rep.References[issueRef1].Data.Assignees).To(Equal(issue1RefData.Assignees))
			})

			When("pushed reference label include a negated entry", func() {
				It("should remove the negated entry from the reference", func() {
					refs = []*types.PushedReference{{
						Name: issueRef1,
						Fee:  "1",
						Data: &remotetypes.ReferenceData{IssueFields: remotetypes.IssueFields{Labels: &[]string{"-label1"}}},
					}}
					err = gitpush.NewContract().Init(logic, &txns.TxPush{
						TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
						Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: creator},
					}, 0).Exec()
					Expect(err).To(BeNil())
					rep := logic.RepoKeeper().Get(repo)
					Expect(rep.References[issueRef1].Data.Labels).To(HaveLen(1))
					Expect(rep.References[issueRef1].Data.Labels).To(ContainElement(issue1RefData.Labels[1]))
				})
			})

			When("pushed reference assignee include a negated entry", func() {
				It("should remove the negated entry from the reference", func() {
					refs = []*types.PushedReference{{
						Name: issueRef1,
						Fee:  "1",
						Data: &remotetypes.ReferenceData{IssueFields: remotetypes.IssueFields{Assignees: &[]string{"-key1"}}}}}
					err = gitpush.NewContract().Init(logic, &txns.TxPush{
						TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
						Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: creator},
					}, 0).Exec()
					Expect(err).To(BeNil())
					rep := logic.RepoKeeper().Get(repo)
					Expect(rep.References[issueRef1].Data.Assignees).To(HaveLen(1))
					Expect(rep.References[issueRef1].Data.Assignees).To(ContainElement(issue1RefData.Assignees[1]))
				})
			})
		})

		When("pushed reference is a merge request reference", func() {
			It("should add new proposal", func() {
				ref := plumbing.MakeMergeRequestReference(1)
				mr := remotetypes.MergeRequestFields{BaseBranch: "master", BaseBranchHash: "hash1", TargetBranch: "dev", TargetBranchHash: "hash1"}
				refs = []*types.PushedReference{{Name: ref, Data: &remotetypes.ReferenceData{MergeRequestFields: mr}, Fee: "1", Value: "1"}}
				err = gitpush.NewContract().Init(logic, &txns.TxPush{
					TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
					Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Has(ref)).To(BeTrue())
				Expect(rep.Proposals).To(HaveLen(1))
				Expect(rep.Proposals.Has(mergerequest.MakeMergeRequestProposalID(1))).To(BeTrue())
				prop := rep.Proposals.Get(mergerequest.MakeMergeRequestProposalID(1))
				Expect(prop.ActionData[constants.ActionDataKeyBaseBranch]).To(Equal(util.Bytes(mr.BaseBranch)))
				Expect(prop.ActionData[constants.ActionDataKeyBaseHash]).To(Equal(util.Bytes(mr.BaseBranchHash)))
				Expect(prop.ActionData[constants.ActionDataKeyTargetBranch]).To(Equal(util.Bytes(mr.TargetBranch)))
				Expect(prop.ActionData[constants.ActionDataKeyTargetBranch]).To(Equal(util.Bytes(mr.TargetBranch)))
			})

			When("reference is not new", func() {
				ref := plumbing.MakeMergeRequestReference(1)

				BeforeEach(func() {
					logic.RepoKeeper().Update(repo, &state.Repository{
						Config: state.DefaultRepoConfig, References: map[string]*state.Reference{ref: {Nonce: 1, Creator: creator}},
					})

					mr := remotetypes.MergeRequestFields{BaseBranch: "master", BaseBranchHash: "hash1", TargetBranch: "dev", TargetBranchHash: "hash1"}
					refs = []*types.PushedReference{{Name: ref, Data: &remotetypes.ReferenceData{MergeRequestFields: mr}, Fee: "1", Value: "1"}}
					err = gitpush.NewContract().Init(logic, &txns.TxPush{
						TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
						Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID, PusherAddress: sender.Addr()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should not deduct value (proposer fee)", func() {
					acct := logic.AccountKeeper().Get(sender.Addr())
					Expect(acct.Balance).To(Equal(util.String("9")))
				})
			})

			When("pushed reference payload as Close=true", func() {
				ref := plumbing.MakeMergeRequestReference(1)

				BeforeEach(func() {
					logic.RepoKeeper().Update(repo, &state.Repository{
						Config:     state.DefaultRepoConfig,
						References: map[string]*state.Reference{ref: {Nonce: 1, Creator: creator, Data: &state.ReferenceData{}}},
					})

					mr := remotetypes.MergeRequestFields{BaseBranch: "master", BaseBranchHash: "hash1", TargetBranch: "dev", TargetBranchHash: "hash1"}
					cls := true
					refs = []*types.PushedReference{{Name: ref, Data: &remotetypes.ReferenceData{MergeRequestFields: mr, Close: &cls}, Fee: "1", Value: "1"}}
					err = gitpush.NewContract().Init(logic, &txns.TxPush{
						TxCommon: &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey()},
						Note:     &types.Note{RepoName: repo, References: refs, PushKeyID: rawPkID, PusherAddress: sender.Addr()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should set reference as closed", func() {
					rep := logic.RepoKeeper().Get(repo)
					Expect(rep.References.Get(ref).Data.Closed).To(BeTrue())
				})
			})
		})
	})
})
