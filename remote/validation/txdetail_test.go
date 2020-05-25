package validation_test

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/mr-tron/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var privKey, privKey2 *crypto.Key
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		privKey = crypto.NewKeyFromIntSeed(1)
		privKey2 = crypto.NewKeyFromIntSeed(2)

		mockObjs := testutil.MockLogic(ctrl)
		mockLogic = mockObjs.Logic
		mockRepoKeeper = mockObjs.RepoKeeper
		mockPushKeyKeeper = mockObjs.PushKeyKeeper
		mockAcctKeeper = mockObjs.AccountKeeper
		mockNSKeeper = mockObjs.NamespaceKeeper
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxDetail", func() {
		It("should return nil when no error ", func() {
			detail := &types.TxDetail{
				PushKeyID: privKey.PushAddr().String(),
				Nonce:     9,
				Fee:       "1",
			}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = privKey.PubKey().ToPublicKey()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = validation.CheckTxDetail(detail, mockLogic, 0)
			Expect(err).To(BeNil())
		})
	})

	Describe(".CheckTxDetailSanity", func() {
		It("should return error when push key is unset", func() {
			detail := &types.TxDetail{}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:pkID, msg:push key id is required"))
		})

		It("should return error when push key is not valid", func() {
			detail := &types.TxDetail{PushKeyID: "invalid_key"}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:pkID, msg:push key id is not valid"))
		})

		It("should return error when nonce is not set", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:nonce, msg:nonce is required"))
		})

		It("should return error when fee is not set", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: ""}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:fee, msg:fee is required"))
		})

		It("should return error when value is set for non-merge request reference", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: "1", Value: "1", Reference: "refs/heads/master"}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:value, msg:field not expected"))
		})

		It("should return error when fee is not numeric", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: "1_invalid"}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:fee, msg:fee must be numeric"))
		})

		It("should return error when signature is malformed", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: "1", Signature: "0x_invalid"}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:sig, msg:signature format is not valid"))
		})

		It("should return error when merge proposal ID is not numeric", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "invalid",
			}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:mergeID, msg:merge proposal id must be numeric"))
		})

		It("should return error when merge proposal ID surpasses 8 bytes", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "1234567890",
			}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:mergeID, msg:merge proposal id exceeded 8 bytes limit"))
		})

		It("should return no error", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "12",
			}
			err := validation.CheckTxDetailSanity(detail, 0)
			Expect(err).To(BeNil())
		})
	})

	Describe(".CheckTxDetailConsistency", func() {
		It("should return error when push key is unknown", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(state.BarePushKey())
			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:pkID, msg:push key not found"))
		})

		It("should return error when repo namespace and push key scopes are set but namespace does not exist", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo1", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"r/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(state.BareNamespace())

			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:namespace, msg:namespace (ns1) is unknown"))
		})

		It("should return scope error when key scope is r/repo1 and tx repo=repo2 and namespace is unset", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: ""}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"r/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return scope error when key scope is ns1/repo1 and tx repo=repo2 and namespace=ns1", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"ns1/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			ns := state.BareNamespace()
			ns.Domains["ns1"] = "real-repo"
			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(ns)

			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return scope error when key scope is ns1/ and tx repo=repo2 and namespace=ns2", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"ns1/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			ns := state.BareNamespace()
			ns.Domains["ns1"] = "real-repo"
			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(ns)

			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return error when nonce is not greater than push key owner account nonce", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 10
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:nonce, msg:nonce (9) must be greater than current key owner nonce (10)"))
		})

		When("merge proposal ID is set", func() {
			It("should return error when the proposal does not exist", func() {
				detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, MergeProposalID: "100"}

				pk := state.BarePushKey()
				pk.Address = privKey.Addr()
				mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

				acct := state.BareAccount()
				acct.Nonce = 8
				mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

				mockRepoKeeper.EXPECT().Get(detail.RepoName).Return(state.BareRepository())

				err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:mergeID, msg:merge proposal not found"))
			})

			It("should return error when the proposal is not a merge request", func() {
				detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, MergeProposalID: "100"}

				pk := state.BarePushKey()
				pk.Address = privKey.Addr()
				mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

				acct := state.BareAccount()
				acct.Nonce = 8
				mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

				repoState := state.BareRepository()
				repoState.Proposals["100"] = &state.RepoProposal{Action: 100000}
				mockRepoKeeper.EXPECT().Get(detail.RepoName).Return(repoState)

				err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:mergeID, msg:proposal is not a merge request"))
			})

			It("should return error when the proposal creator is not the push key owner", func() {
				detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, MergeProposalID: "100"}

				pk := state.BarePushKey()
				pk.Address = privKey.Addr()
				mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

				acct := state.BareAccount()
				acct.Nonce = 8
				mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

				repoState := state.BareRepository()
				repoState.Proposals["100"] = &state.RepoProposal{Action: txns.MergeRequestProposalAction, Creator: privKey2.Addr().String()}
				mockRepoKeeper.EXPECT().Get(detail.RepoName).Return(repoState)

				err := validation.CheckTxDetailConsistency(detail, mockLogic, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:mergeID, msg:merge error: signer did not create the proposal"))
			})
		})

		It("should return error when signature could not be verified", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = crypto.BytesToPublicKey([]byte("bad key"))
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:sig, msg:signature is not valid"))
		})

		It("should return nil when signature is valid", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = privKey.PubKey().ToPublicKey()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = validation.CheckTxDetailConsistency(detail, mockLogic, 0)
			Expect(err).To(BeNil())
		})
	})

})
