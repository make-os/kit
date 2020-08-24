package validation_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	mr "github.com/make-os/lobe/logic/contracts/mergerequest"
	"github.com/make-os/lobe/mocks"
	plumbing2 "github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Merge", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		mockObjs := testutil.MockLogic(ctrl)
		mockLogic = mockObjs.Logic
		mockRepoKeeper = mockObjs.RepoKeeper
		mockPushKeyKeeper = mockObjs.PushKeyKeeper
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckMergeCompliance", func() {
		When("pushed reference is not a branch", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/others/name", Data: "0001hash"}}
				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: pushed reference must be a branch"))
			})
		})

		When("target merge proposal does not exist", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetState().Return(state.BareRepository())
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}
				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal was not found"))
			})
		})

		When("signer did not create the proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Creator = "address_of_creator"
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{Address: "address_xyz"})

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: push key owner did not create the proposal"))
			})
		})

		When("unable to check whether proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), state.BareRepoProposal())
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).
					Return(false, fmt.Errorf("error"))

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: error"))
			})
		})

		When("target merge proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), state.BareRepoProposal())
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(true, nil)

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal is already closed"))
			})
		})

		When("target merge proposal's base branch name does not match the pushed branch name", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string]util.Bytes{
					constants.ActionDataKeyBaseBranch: util.ToBytes("release"),
				}
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(false, nil)

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: pushed branch name and proposal base branch name must match"))
			})
		})

		When("target merge proposal outcome has not been decided", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.ActionData = map[string]util.Bytes{
					constants.ActionDataKeyBaseBranch: []byte("master"),
				}
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(false, nil)

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal is undecided"))
			})
		})

		When("target merge proposal outcome has been decided but not approved", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.ActionData = map[string]util.Bytes{
					constants.ActionDataKeyBaseBranch: []byte("master"),
				}
				prop.Outcome = 3
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(false, nil)

				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}

				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal was not accepted"))
			})
		})

		When("merge proposal target hash does not match the expected target hash", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string]util.Bytes{
					constants.ActionDataKeyBaseBranch: []byte("master"),
					constants.ActionDataKeyBaseHash:   []byte("abc"),
					constants.ActionDataKeyTargetHash: []byte("target_xyz"),
				}
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)
				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(false, nil)
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "0001hash"}}
				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: pushed commit did not match merge proposal target hash"))
			})
		})

		When("pushed commit hash matches the expected merge proposal target hash", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string]util.Bytes{
					constants.ActionDataKeyBaseBranch: []byte("master"),
					constants.ActionDataKeyBaseHash:   []byte("abc"),
					constants.ActionDataKeyTargetHash: []byte("000hash"),
				}
				repoState.Proposals.Add(mr.MakeMergeRequestProposalID("1"), prop)
				repo.EXPECT().GetState().Return(repoState)
				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", mr.MakeMergeRequestProposalID("1")).Return(false, nil)
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "000hash"}}
				err = validation.CheckMergeCompliance(repo, change, "1", "push_key_id", mockLogic)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
