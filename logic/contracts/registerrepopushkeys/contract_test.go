package registerrepopushkeys_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts"
	"gitlab.com/makeos/mosdef/logic/contracts/registerrepopushkeys"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("UpsertOwnerContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
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
			ct := registerrepopushkeys.NewContract(nil)
			Expect(ct.CanExec(core.TxTypeRepoProposalRegisterPushKey)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeHostTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", DelegatorCommission: 10})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.Voter = state.VoterOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = registerrepopushkeys.NewContract(&contracts.SystemContracts).Init(logic, &core.TxRepoProposalRegisterPushKey{
					TxCommon:         &core.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &core.TxProposalCommon{RepoName: repoName, Value: proposalFee, ProposalID: propID},
					FeeMode:          state.FeeModePusherPays,
					FeeCap:           "0",
					KeyIDs:           []string{"push1_abc"},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})
	})

	Describe(".Apply", func() {
		var repoUpd *state.Repository

		BeforeEach(func() {
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
		})

		When("2 ids were provided in action data", func() {
			BeforeEach(func() {
				proposal := &state.RepoProposal{ActionData: map[string][]byte{
					constants.ActionDataKeyPolicies: util.ToBytes([]*state.Policy{{Action: "act", Subject: "sub", Object: "obj"}}),
					constants.ActionDataKeyIDs:      util.ToBytes([]string{"push1_abc", "push1_xyz"}),
					constants.ActionDataKeyFeeMode:  util.ToBytes(state.FeeModePusherPays),
				}}
				err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repoUpd,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
			})

			It("should add 2 contributors with same policies, feeMode, feeCap, feeUsed fields", func() {
				Expect(repoUpd.Contributors).To(HaveLen(2))
				Expect(repoUpd.Contributors["push1_abc"]).To(Equal(repoUpd.Contributors["push1_xyz"]))
			})
		})

		When("feeMode is FeeModeRepoPaysCapped", func() {
			BeforeEach(func() {
				proposal := &state.RepoProposal{ActionData: map[string][]byte{
					constants.ActionDataKeyPolicies: util.ToBytes([]*state.Policy{{Action: "act", Subject: "sub", Object: "obj"}}),
					constants.ActionDataKeyIDs:      util.ToBytes([]string{"push1_abc"}),
					constants.ActionDataKeyFeeMode:  util.ToBytes(state.FeeModeRepoPaysCapped),
					constants.ActionDataKeyFeeCap:   util.ToBytes(util.String("100")),
				}}
				err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repoUpd,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
			})

			It("should set feeCap field", func() {
				Expect(repoUpd.Contributors).To(HaveLen(1))
				Expect(repoUpd.Contributors["push1_abc"].FeeCap).To(Equal(util.String("100")))
			})
		})

		When("feeMode is not FeeModeRepoPaysCapped", func() {
			BeforeEach(func() {
				proposal := &state.RepoProposal{ActionData: map[string][]byte{
					constants.ActionDataKeyPolicies: util.ToBytes([]*state.Policy{{Action: "act", Subject: "sub", Object: "obj"}}),
					constants.ActionDataKeyIDs:      util.ToBytes([]string{"push1_abc"}),
					constants.ActionDataKeyFeeMode:  util.ToBytes(state.FeeModeRepoPays),
					constants.ActionDataKeyFeeCap:   util.ToBytes(util.String("100")),
				}}
				err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repoUpd,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
			})

			Specify("that feeCap field is zero", func() {
				Expect(repoUpd.Contributors).To(HaveLen(1))
				Expect(repoUpd.Contributors["push1_abc"].FeeCap).To(Equal(util.String("0")))
			})
		})

		When("namespace 'ns' is provided in action data", func() {
			var ns = "my_namespace"
			var nsObj *state.Namespace
			var proposal *state.RepoProposal

			When("the target namespace does not exist", func() {
				BeforeEach(func() {
					proposal = &state.RepoProposal{ActionData: map[string][]byte{
						constants.ActionDataKeyPolicies:  util.ToBytes([]*state.Policy{}),
						constants.ActionDataKeyIDs:       util.ToBytes([]string{"push1_abc"}),
						constants.ActionDataKeyFeeMode:   util.ToBytes(state.FeeModeRepoPays),
						constants.ActionDataKeyFeeCap:    util.ToBytes(util.String("100")),
						constants.ActionDataKeyNamespace: util.ToBytes("other_namespace"),
					}}
				})

				Specify("that it panicked", func() {
					Expect(func() {
						err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
							Proposal:    proposal,
							Repo:        repoUpd,
							ChainHeight: 0,
						})
						Expect(err).To(BeNil())
					}).To(Panic())
				})
			})

			When("the target namespace exist", func() {
				BeforeEach(func() {
					nsObj = state.BareNamespace()
					nsObj.Owner = "repo1"
					logic.NamespaceKeeper().Update(util.HashNamespace(ns), nsObj)
					proposal = &state.RepoProposal{ActionData: map[string][]byte{
						constants.ActionDataKeyPolicies:  util.ToBytes([]*state.Policy{}),
						constants.ActionDataKeyIDs:       util.ToBytes([]string{"push1_abc"}),
						constants.ActionDataKeyFeeMode:   util.ToBytes(state.FeeModeRepoPays),
						constants.ActionDataKeyNamespace: util.ToBytes(ns),
					}}
					err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
						Keepers:     logic,
						Proposal:    proposal,
						Repo:        repoUpd,
						ChainHeight: 0,
					})
					Expect(err).To(BeNil())
				})

				It("should add 1 contributor to the repo", func() {
					Expect(repoUpd.Contributors).To(HaveLen(1))
				})

				It("should add 1 contributor to the namespace", func() {
					nsKey := util.HashNamespace(ns)
					nsObj := logic.NamespaceKeeper().Get(nsKey)
					Expect(nsObj.Contributors).To(HaveLen(1))
					Expect(nsObj.Contributors["push1_abc"]).ToNot(BeNil())
				})
			})
		})

		When("namespaceOnly 'nso' is provided in action data", func() {
			var ns = "my_namespace"
			var nsObj *state.Namespace
			var proposal *state.RepoProposal

			When("the target namespace does not exist", func() {
				BeforeEach(func() {
					proposal = &state.RepoProposal{ActionData: map[string][]byte{
						constants.ActionDataKeyPolicies:      util.ToBytes([]*state.Policy{}),
						constants.ActionDataKeyIDs:           util.ToBytes([]string{"push1_abc"}),
						constants.ActionDataKeyFeeMode:       util.ToBytes(state.FeeModeRepoPays),
						constants.ActionDataKeyFeeCap:        util.ToBytes(util.String("100")),
						constants.ActionDataKeyNamespaceOnly: util.ToBytes("other_namespace"),
					}}
				})

				Specify("that it panicked", func() {
					Expect(func() {
						err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
							Keepers:     logic,
							Proposal:    proposal,
							Repo:        repoUpd,
							ChainHeight: 0,
						})
						Expect(err).To(BeNil())
					}).To(Panic())
				})
			})

			When("the target namespace exist", func() {
				BeforeEach(func() {
					nsObj = state.BareNamespace()
					nsObj.Owner = "repo1"
					logic.NamespaceKeeper().Update(util.HashNamespace(ns), nsObj)
					proposal = &state.RepoProposal{ActionData: map[string][]byte{
						constants.ActionDataKeyPolicies:      util.ToBytes([]*state.Policy{}),
						constants.ActionDataKeyIDs:           util.ToBytes([]string{"push1_abc"}),
						constants.ActionDataKeyFeeMode:       util.ToBytes(state.FeeModeRepoPays),
						constants.ActionDataKeyNamespaceOnly: util.ToBytes(ns),
					}}
					err = registerrepopushkeys.NewContract(nil).Apply(&core.ProposalApplyArgs{
						Keepers:     logic,
						Proposal:    proposal,
						Repo:        repoUpd,
						ChainHeight: 0,
					})
					Expect(err).To(BeNil())
				})

				It("should add no (0) contributor to the repo", func() {
					Expect(repoUpd.Contributors).To(HaveLen(0))
				})

				It("should add 1 contributor to the namespace", func() {
					nsKey := util.HashNamespace(ns)
					nsObj := logic.NamespaceKeeper().Get(nsKey)
					Expect(nsObj.Contributors).To(HaveLen(1))
					Expect(nsObj.Contributors["push1_abc"]).ToNot(BeNil())
				})
			})
		})
	})
})
