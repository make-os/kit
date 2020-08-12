package gitcmd

import (
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	types2 "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("GitVerify", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		key = crypto.NewKeyFromIntSeed(1)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GitVerifyCmd", func() {
		It("should return err when unable to read signature from file", func() {
			args := &GitVerifyArgs{Args: []string{"", "", "", "", "unknown_file.txt"}, StdErr: ioutil.Discard, StdOut: ioutil.Discard}
			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read sig file: open unknown_file.txt: no such file or directory"))
		})

		It("should return err when unable to decode signature", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			_, err = f.WriteString("invalid signature")
			Expect(err).To(BeNil())
			f.Close()

			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()}, StdErr: ioutil.Discard,
				StdOut: ioutil.Discard, PemDecoder: pem.Decode}
			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("malformed signature. Expected PEM encoded signature"))
		})

		It("should return err when unable to decode signature header to TxDetail", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			f.Close()

			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()}, StdErr: ioutil.Discard, StdOut: ioutil.Discard}
			args.PemDecoder = func(data []byte) (p *pem.Block, rest []byte) {
				return &pem.Block{Headers: map[string]string{}}, nil
			}

			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid header: 'pkID' is required"))
		})

		It("should return err when unable to get local repository at current working directory", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			f.Close()

			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()}, StdErr: ioutil.Discard, StdOut: ioutil.Discard}
			args.PemDecoder = func(data []byte) (p *pem.Block, rest []byte) {
				return &pem.Block{Headers: map[string]string{
					"pkID": key.PushAddr().String(),
				}}, nil
			}
			args.RepoGetter = func(path string) (types2.LocalRepo, error) {
				return nil, fmt.Errorf("error")
			}

			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo: error"))
		})

		It("should return err when failed to unlock the signing key", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			f.Close()

			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()}, StdErr: ioutil.Discard, StdOut: ioutil.Discard}
			args.PemDecoder = func(data []byte) (p *pem.Block, rest []byte) {
				return &pem.Block{Headers: map[string]string{
					"pkID": key.PushAddr().String(),
				}}, nil
			}
			args.RepoGetter = func(path string) (types2.LocalRepo, error) {
				return mockRepo, nil
			}
			args.PushKeyUnlocker = func(cfg *config.AppConfig, args *common.UnlockKeyArgs) (types.StoredKey, error) {
				return nil, fmt.Errorf("error")
			}

			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should return err when unable to verify signature", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			f.Close()

			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()},
				StdErr: ioutil.Discard,
				StdOut: ioutil.Discard,
				StdIn:  &testutil.WrapReadCloser{Buf: []byte("data"), Err: io.EOF},
			}

			args.PemDecoder = func(data []byte) (p *pem.Block, rest []byte) {
				return &pem.Block{
					Bytes:   []byte("invalid signature"),
					Headers: map[string]string{"pkID": key.PushAddr().String()},
				}, nil
			}

			args.RepoGetter = func(path string) (types2.LocalRepo, error) {
				return mockRepo, nil
			}

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetKey().Return(key)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, args *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}

			err := GitVerifyCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signature is not valid"))
		})

		It("should return no error when signature is valid", func() {
			f, _ := ioutil.TempFile(os.TempDir(), "")
			f.Close()

			gitObjectData := []byte("data")
			args := &GitVerifyArgs{Args: []string{"", "", "", "", f.Name()},
				StdErr: ioutil.Discard,
				StdOut: ioutil.Discard,
				StdIn:  &testutil.WrapReadCloser{Buf: gitObjectData, Err: io.EOF},
			}

			// Create signature
			txDetail := &types2.TxDetail{PushKeyID: key.PushAddr().String()}
			sig, err := key.PrivKey().Sign(gitObjectData)
			Expect(err).To(BeNil())

			args.PemDecoder = func(data []byte) (p *pem.Block, rest []byte) {
				return &pem.Block{
					Bytes:   sig,
					Headers: txDetail.GetGitSigPEMHeader(),
				}, nil
			}

			args.RepoGetter = func(path string) (types2.LocalRepo, error) {
				return mockRepo, nil
			}

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetKey().Return(key)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, args *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}

			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			err = GitVerifyCmd(cfg, args)
			Expect(err).To(BeNil())
		})
	})
})
