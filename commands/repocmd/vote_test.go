package repocmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	types2 "github.com/themakeos/lobe/api/types"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	kstypes "github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("VoteCmd", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".VoteCmd", func() {
		It("should return error when failed to unlock account", func() {
			args := &VoteArgs{SigningKey: "1", SigningKeyPass: "pass"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyAddrOrIdx).To(Equal(args.SigningKey))
				Expect(a.Passphrase).To(Equal(args.SigningKeyPass))
				return nil, fmt.Errorf("error")
			}
			err := VoteCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when nonce is 0 and it failed to fetch next nonce", func() {
			args := &VoteArgs{SigningKey: "1", SigningKeyPass: "pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "", fmt.Errorf("error")
			}
			err := VoteCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when to cast vote", func() {
			args := &VoteArgs{RepoName: "repo1", Fee: 1.2, Vote: 1, SigningKey: "1", SigningKeyPass: "pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.VoteCreator = func(req *types2.RepoVoteBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				Expect(req.RepoName).To(Equal(args.RepoName))
				Expect(req.ProposalID).To(Equal(args.ProposalID))
				Expect(req.Vote).To(Equal(args.Vote))
				Expect(req.Nonce).To(Equal(uint64(2)))
				Expect(req.Fee).To(Equal(1.2))
				return "", fmt.Errorf("error")
			}
			err := VoteCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to cast vote: error"))
		})

		It("should return nil on success", func() {
			args := &VoteArgs{RepoName: "repo1", Vote: 1, Fee: 1.2, SigningKey: "1", SigningKeyPass: "pass", Stdout: ioutil.Discard}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.VoteCreator = func(req *types2.RepoVoteBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient client.Client, remoteClients []restclient.Client) error {
				return nil
			}
			err := VoteCmd(cfg, args)
			Expect(err).To(BeNil())
		})

		It("should return error when tx tracker returns error", func() {
			args := &VoteArgs{RepoName: "repo1", Vote: 1, Fee: 1.2, SigningKey: "1", SigningKeyPass: "pass", Stdout: ioutil.Discard}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				return "2", nil
			}
			args.VoteCreator = func(req *types2.RepoVoteBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient client.Client, remoteClients []restclient.Client) error {
				return fmt.Errorf("error")
			}
			err := VoteCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
