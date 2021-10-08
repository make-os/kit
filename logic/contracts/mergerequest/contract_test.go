package mergerequest_test

import (
	"os"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/mergerequest"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestMergeRequest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MergeRequest Suite")
}

var _ = Describe("MergeRequestContract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)
	var key2 = ed25519.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&state.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
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
			data := &mergerequest.Data{}
			ct := mergerequest.NewContract(data)
			Expect(ct.CanExec(txns.TxTypeMergeRequestProposalAction)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeRepoProposalSendFee)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repo *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             "10",
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
			repo = state.BareRepository()
			repo.Config = state.DefaultRepoConfig
			repo.Config.Gov.Voter = pointer.ToInt(int(state.VoterOwner))
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			id := "1"

			BeforeEach(func() {
				repo.AddOwner(sender.Addr().String(), &state.RepoOwner{})

				err = mergerequest.NewContract(&mergerequest.Data{
					Repo:             repo,
					RepoName:         repoName,
					ProposalID:       id,
					ProposerFee:      proposalFee,
					Fee:              "1.5",
					CreatorAddress:   sender.Addr(),
					BaseBranch:       "base",
					BaseBranchHash:   "baseHash",
					TargetBranch:     "target",
					TargetBranchHash: "targetHash",
				}).Init(logic, nil, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				Expect(repo.Proposals).To(HaveLen(1))
				propID := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
			})

			Specify("that no fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("10"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				propID := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal(id))
			})
		})

		When("sender is not the only owner", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")
			id := "1"

			BeforeEach(func() {
				repo.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repo.AddOwner(key2.Addr().String(), &state.RepoOwner{})

				err = mergerequest.NewContract(&mergerequest.Data{
					Repo:             repo,
					RepoName:         repoName,
					ProposalID:       id,
					ProposerFee:      proposalFee,
					Fee:              "1.5",
					CreatorAddress:   sender.Addr(),
					BaseBranch:       "base",
					BaseBranchHash:   "baseHash",
					TargetBranch:     "target",
					TargetBranchHash: "targetHash",
				}).Init(logic, nil, curHeight).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				Expect(repo.Proposals).To(HaveLen(1))
				propID := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(0)))
			})

			Specify("that no fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("10"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				propID := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal(id))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(
					util.PtrStrToUInt64(repo.Config.Gov.PropDuration) + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("the proposal already exist and is not finalized", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")
			id := "1"

			BeforeEach(func() {
				repo.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repo.Proposals.Add(mergerequest.MakeMergeRequestProposalID(id), &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyBaseBranch:   []byte("base"),
						constants.ActionDataKeyBaseHash:     []byte("baseHash"),
						constants.ActionDataKeyTargetBranch: []byte("target"),
						constants.ActionDataKeyTargetHash:   []byte("targetHash"),
					},
				})

				err = mergerequest.NewContract(&mergerequest.Data{
					Repo:             repo,
					RepoName:         repoName,
					ProposalID:       id,
					ProposerFee:      proposalFee,
					Fee:              "1.5",
					CreatorAddress:   sender.Addr(),
					BaseBranch:       "base2",
					BaseBranchHash:   "baseHash2",
					TargetBranch:     "target2",
					TargetBranchHash: "targetHash2",
				}).Init(logic, nil, curHeight).Exec()
				Expect(err).To(BeNil())
			})

			It("should not add a new proposal to the repo", func() {
				Expect(repo.Proposals).To(HaveLen(1))
			})

			It("should update proposal action data", func() {
				Expect(repo.Proposals).To(HaveLen(1))
				id := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyBaseBranch]).To(Equal(util.Bytes("base2")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyBaseHash]).To(Equal(util.Bytes("baseHash2")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyTargetBranch]).To(Equal(util.Bytes("target2")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyTargetHash]).To(Equal(util.Bytes("targetHash2")))
			})
		})

		When("the proposal already exist and is finalized", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")
			id := "1"

			BeforeEach(func() {
				repo.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repo.Proposals.Add(mergerequest.MakeMergeRequestProposalID(id), &state.RepoProposal{
					Outcome: state.ProposalOutcomeAccepted,
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyBaseBranch:   []byte("base"),
						constants.ActionDataKeyBaseHash:     []byte("baseHash"),
						constants.ActionDataKeyTargetBranch: []byte("target"),
						constants.ActionDataKeyTargetHash:   []byte("targetHash"),
					},
				})

				err = mergerequest.NewContract(&mergerequest.Data{
					Repo:             repo,
					RepoName:         repoName,
					ProposalID:       id,
					ProposerFee:      proposalFee,
					Fee:              "1.5",
					CreatorAddress:   sender.Addr(),
					BaseBranch:       "base2",
					BaseBranchHash:   "baseHash2",
					TargetBranch:     "target2",
					TargetBranchHash: "targetHash2",
				}).Init(logic, nil, curHeight).Exec()
				Expect(err).To(BeNil())
			})

			It("should not add a new proposal to the repo", func() {
				Expect(repo.Proposals).To(HaveLen(1))
			})

			It("should not update proposal action data", func() {
				Expect(repo.Proposals).To(HaveLen(1))
				id := mergerequest.MakeMergeRequestProposalID(id)
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyBaseBranch]).To(Equal(util.Bytes("base")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyBaseHash]).To(Equal(util.Bytes("baseHash")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyTargetBranch]).To(Equal(util.Bytes("target")))
				Expect(repo.Proposals.Get(id).ActionData[constants.ActionDataKeyTargetHash]).To(Equal(util.Bytes("targetHash")))
			})
		})
	})
})
