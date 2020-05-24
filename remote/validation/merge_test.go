package validation_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Validation", func() {
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
				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/others/name", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{Address: "address_xyz"})

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, fmt.Errorf("error"))

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(true, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("release"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
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
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				prop.Outcome = 3
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal was not accepted"))
			})
		})

		When("unable to get pushed commit", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(nil, fmt.Errorf("error"))

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: error"))
			})
		})

		When("pushed commit is a merge commit (has multiple parents) but proposal target hash is not a parent", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyTargetHash: util.ToBytes("target_xyz"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				pushedCommit := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().NumParents().Return(2)
				pushedCommitParent := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().Parent(0).Return(pushedCommitParent, nil)
				pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
				pushedCommit.EXPECT().IsParent("target_xyz").Return(false, nil)
				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target hash is not a parent of the merge commit"))
			})
		})

		When("pushed commit modified worktree history of parent", func() {
			When("tree hash is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockLocalRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().GetState().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					pushedCommit := mocks.NewMockCommit(ctrl)
					pushedCommit.EXPECT().NumParents().Return(1)
					pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)

					change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

					oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
					err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: pushed commit must not modify target branch history"))
				})
			})

			When("author is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockLocalRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().GetState().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					pushedCommit := mocks.NewMockCommit(ctrl)
					pushedCommit.EXPECT().NumParents().Return(1)
					pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
					treeHash := plumbing2.MakeCommitHash("hash")
					pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					pushedCommit.EXPECT().GetAuthor().Return(author)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing2.MakeCommitHash("hash")
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author2@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)
					pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)

					change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

					oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
					err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: pushed commit must not modify target branch history"))
				})
			})

			When("committer is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockLocalRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().GetState().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					pushedCommit := mocks.NewMockCommit(ctrl)
					pushedCommit.EXPECT().NumParents().Return(1)
					pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
					treeHash := plumbing2.MakeCommitHash("hash")
					pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					pushedCommit.EXPECT().GetAuthor().Return(author)
					committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
					pushedCommit.EXPECT().GetCommitter().Return(committer)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing2.MakeCommitHash("hash")
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)
					committer = &object.Signature{Name: "committer1", Email: "committer2@email.com"}
					targetCommit.EXPECT().GetCommitter().Return(committer)
					pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)

					change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

					err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: pushed commit must not modify target branch history"))
				})
			})
		})

		When("old pushed branch hash is different from old branch hash described in the merge proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("xyz"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "abc"}

				pushedCommit := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().NumParents().Return(1)
				pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
				treeHash := plumbing2.MakeCommitHash("hash")
				pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				pushedCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				pushedCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				treeHash = plumbing2.MakeCommitHash("hash")
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target merge proposal base branch hash is stale or invalid"))
			})
		})

		When("merge proposal target hash does not match the expected target hash", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("abc"),
					constants.ActionDataKeyTargetHash: util.ToBytes("target_xyz"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "abc"}

				pushedCommit := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().NumParents().Return(1)
				pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
				treeHash := plumbing2.MakeCommitHash("hash")
				pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				pushedCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				pushedCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				targetHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("target_abc"))
				targetCommit.EXPECT().GetHash().Return(targetHash)
				treeHash = plumbing2.MakeCommitHash("hash")
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target commit hash and the merge proposal target hash must match"))
			})
		})

		When("pushed commit hash matches proposal target hash and pushed commit history is compliant with merge proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				propTargetHash := plumbing2.MakeCommitHash(util.RandString(20))
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("abc"),
					constants.ActionDataKeyTargetHash: util.ToBytes(propTargetHash.String()),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "abc"}

				pushedCommit := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().NumParents().Return(1)
				pushedCommit.EXPECT().GetHash().Return(propTargetHash).Times(2)
				pushedCommit.EXPECT().Parent(0).Return(nil, nil)
				treeHash := plumbing2.MakeCommitHash("hash")
				pushedCommit.EXPECT().GetTreeHash().Return(treeHash).Times(2)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				pushedCommit.EXPECT().GetAuthor().Return(author).Times(2)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				pushedCommit.EXPECT().GetCommitter().Return(committer).Times(2)

				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("pushed commit history is compliant with merge proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				propTargetHash := plumbing2.MakeCommitHash(util.RandString(20))
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("abc"),
					constants.ActionDataKeyTargetHash: util.ToBytes(propTargetHash.String()),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().GetState().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})
				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				change := &core.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &plumbing2.Obj{Name: "refs/heads/unknown", Data: "abc"}

				pushedCommit := mocks.NewMockCommit(ctrl)
				pushedCommit.EXPECT().NumParents().Return(1)
				pushedCommit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("push_commit_hash"))
				treeHash := plumbing2.MakeCommitHash("hash")
				pushedCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				pushedCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				pushedCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				targetCommit.EXPECT().GetHash().Return(propTargetHash)
				treeHash = plumbing2.MakeCommitHash("hash")
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				pushedCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(pushedCommit, nil)

				err = validation.CheckMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
