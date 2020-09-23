package push_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht/announcer"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/params"
	plumbing2 "github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/policy"
	"github.com/make-os/lobe/remote/push"
	pushtypes "github.com/make-os/lobe/remote/push/types"
	repo3 "github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/remote/server"
	remotetestutil "github.com/make-os/lobe/remote/testutil"
	"github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	mocks2 "github.com/make-os/lobe/util/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

func TestPush(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Push Suite")
}

var _ = Describe("BasicHandler", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo types.LocalRepo
	var mockRemoteSrv *mocks.MockRemoteServer
	var svr core.RemoteServer
	var handler *push.BasicHandler
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var repoName string
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter
	var mockDHT *mocks.MockDHT
	var mockGitRcvCmd *mocks2.MockCmd
	var mockPushPool *mocks.MockPushPool

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		remotetestutil.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockDHT = mocks.NewMockDHT(ctrl)
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeRepoName, gomock.Any())
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeGit, gomock.Any())

		mockLogic = mocks.NewMockLogic(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockGitRcvCmd = mocks2.NewMockCmd(ctrl)
		mockPushPool = mocks.NewMockPushPool(ctrl)

		svr = server.New(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)
		mockRemoteSrv = mocks.NewMockRemoteServer(ctrl)
		mockRemoteSrv.EXPECT().Log().Return(cfg.G().Log)
		mockRemoteSrv.EXPECT().GetPushPool().Return(mockPushPool).AnyTimes()

		handler = push.NewHandler(repo, []*types.TxDetail{}, nil, mockRemoteSrv)
	})

	AfterEach(func() {
		svr.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".HandleStream", func() {
		When("unable to get repo old state", func() {
			BeforeEach(func() {
				mockRemoteSrv.EXPECT().GetRepoState(repo).Return(nil, fmt.Errorf("error"))
				err = handler.HandleStream(nil, nil, nil, nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})

		When("packfile is invalid", func() {
			BeforeEach(func() {
				oldState := &plumbing2.State{}
				mockRemoteSrv.EXPECT().GetRepoState(repo).Return(oldState, nil)
				mockGitRcvCmd.EXPECT().SetStderr(gomock.Any())
				err = handler.HandleStream(strings.NewReader("invalid"), nil, mockGitRcvCmd, nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to decode request pack: invalid pkt-len found"))
			})
		})

		When("packfile is valid", func() {
			var packfile io.ReadSeeker

			BeforeEach(func() {
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash := remotetestutil.GetRecentCommitHash(path, "refs/heads/master")
				note := &pushtypes.Note{
					TargetRepo: repo,
					References: []*pushtypes.PushedReference{
						{Name: "refs/heads/master", NewHash: commitHash, OldHash: plumbing.ZeroHash.String()},
					},
				}
				packfile, err = push.MakeReferenceUpdateRequestPack(note)
				Expect(err).To(BeNil())
			})

			When("old state is unset", func() {
				It("should return error when unable to get repo state", func() {
					mockRemoteSrv.EXPECT().GetRepoState(handler.Repo).Return(nil, fmt.Errorf("error"))
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)}, mockGitRcvCmd, nil)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("error"))
				})
			})

			When("authorization failed", func() {
				It("should return error returned from the authorization handler", func() {
					handler.OldState = &plumbing2.State{}
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return fmt.Errorf("auth failed badly")
					}
					mockGitRcvCmd.EXPECT().SetStderr(gomock.Any())
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)}, mockGitRcvCmd, nil)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("authorization: auth failed badly"))
				})
			})

			When("authorization succeeds", func() {
				It("should return no error", func() {
					handler.OldState = &plumbing2.State{}
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error { return nil }
					mockGitRcvCmd.EXPECT().SetStderr(gomock.Any())
					mockGitRcvCmd.EXPECT().ProcessWait()
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)}, mockGitRcvCmd, nil)
					Expect(err).To(BeNil())
				})
			})

			When("git-receive-pack failed", func() {
				It("should return error", func() {
					handler.OldState = &plumbing2.State{}
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error { return nil }
					mockGitRcvCmd.EXPECT().SetStderr(gomock.Any())
					mockGitRcvCmd.EXPECT().ProcessWait().Return(fmt.Errorf("process error"))
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)}, mockGitRcvCmd, nil)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("git-receive-pack: write error: "))
				})
			})
		})
	})

	Describe(".EnsureReferencesHaveTxDetail", func() {
		It("should return error if a reference has no tx detail", func() {
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}
			err := handler.EnsureReferencesHaveTxDetail()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("reference (refs/heads/master) has no transaction information"))
		})

		It("should return no error if a reference has tx detail", func() {
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}
			handler.TxDetails["refs/heads/master"] = &types.TxDetail{}
			err := handler.EnsureReferencesHaveTxDetail()
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleAuthorization", func() {
		var ur *packp.ReferenceUpdateRequest
		BeforeEach(func() {
			handler.Repo.SetState(state.BareRepository())
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}
			ur = &packp.ReferenceUpdateRequest{}
		})

		It("should return error if a reference has no tx detail", func() {
			err = handler.HandleAuthorization(ur)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("reference (refs/heads/master) has no transaction information"))
		})
	})

	Describe(".DoAuth", func() {
		var ur *packp.ReferenceUpdateRequest

		BeforeEach(func() {
			handler.Repo.SetState(state.BareRepository())
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}
			ur = &packp.ReferenceUpdateRequest{}
		})

		Specify("that policy is not checked when tx detail has a merge proposal ID set", func() {
			ref := "refs/heads/master"
			handler.TxDetails[ref] = &types.TxDetail{MergeProposalID: "111"}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: plumbing.ZeroHash})
			policyChecked := false
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				policyChecked = true
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
			Expect(policyChecked).To(BeFalse())
		})

		Specify("that for branch reference with new hash = zero hash, policy is 'PolicyActionDelete'", func() {
			ref := "refs/heads/master"
			handler.TxDetails[ref] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: plumbing.ZeroHash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionDelete))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for merge reference, with non-zero new hash, and "+
			"tx detail has MergeProposalID set, "+
			"policy is 'PolicyActionWrite'", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.PushReader.References[ref] = &push.PackedReferenceObject{}
			handler.TxDetails[ref] = &types.TxDetail{MergeProposalID: "123"}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with non-zero new hash, policy is 'PolicyActionWrite'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.PushReader.References[issueBranch] = &push.PackedReferenceObject{}
			handler.TxDetails[issueBranch] = &types.TxDetail{MergeProposalID: ""}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with zero new hash, policy is 'PolicyActionDelete'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.PushReader.References = map[string]*push.PackedReferenceObject{issueBranch: {}}
			handler.TxDetails[plumbing2.MakeIssueReference(1)] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: plumbing.ZeroHash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionDelete))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with non-zero new hash and"+
			"the reference previously exist and"+
			"tx detail FlagCheckIssueUpdatePolicy is true, "+
			"policy is 'PolicyActionUpdate'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.Repo.GetState().References[issueBranch] = &state.Reference{Hash: []byte("hash")}
			handler.PushReader.References = map[string]*push.PackedReferenceObject{issueBranch: {}}
			handler.TxDetails[issueBranch] = &types.TxDetail{FlagCheckAdminUpdatePolicy: true}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionUpdate))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that policy action is PolicyActionWrite when command is "+
			"an update request and"+
			"reference is an issue reference and"+
			"FlagCheckIssueUpdatePolicy is set in tx detail and"+
			"reference did not previously exist (new reference) and", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.PushReader.References = map[string]*push.PackedReferenceObject{issueBranch: {}}
			handler.TxDetails[issueBranch] = &types.TxDetail{FlagCheckAdminUpdatePolicy: true}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for merge reference with non-zero new hash, policy is 'PolicyActionMergeWrite'", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.TxDetails[ref] = &types.TxDetail{}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for merge reference with newHash=zero, policy is 'PolicyActionMergeDelete'", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.TxDetails[ref] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: plumbing.ZeroHash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionDelete))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for merge reference with newHash=non-zero and "+
			"reference is not new and "+
			"FlagCheckAdminUpdatePolicy is true, "+
			"policy is 'PolicyActionMergeUpdate'", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.TxDetails[ref] = &types.TxDetail{FlagCheckAdminUpdatePolicy: true}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			handler.Repo.GetState().References[ref] = &state.Reference{Hash: []byte("hash")}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionUpdate))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that when policy checker function return error, DoAuth returns the error", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.TxDetails[ref] = &types.TxDetail{FlagCheckAdminUpdatePolicy: true}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				return fmt.Errorf("error")
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("target reference is provided, only the reference is checked", func() {
			It("should check only ref2 when ref2 is the target reference", func() {
				ref, ref2 := "refs/heads/master", "refs/heads/dev"
				handler.TxDetails[ref] = &types.TxDetail{}
				handler.TxDetails[ref2] = &types.TxDetail{}
				hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
				hash2 := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
				ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
				ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref2), New: hash2})
				handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
					Expect(reference).To(Equal(ref2))
					return nil
				}
				err = handler.DoAuth(ur, ref2, false)
				Expect(err).To(BeNil())
			})
		})

		When("ignorePostRefs is true, only the non-post references are checked", func() {
			It("should check only ref3 since it is not a post reference", func() {
				ref, ref2, ref3 := plumbing2.MakeIssueReference(1), plumbing2.MakeMergeRequestReference(1), "refs/heads/dev"
				handler.TxDetails[ref] = &types.TxDetail{}
				handler.TxDetails[ref2] = &types.TxDetail{}
				handler.TxDetails[ref3] = &types.TxDetail{}
				hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
				hash2 := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
				hash3 := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
				ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
				ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref2), New: hash2})
				ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref3), New: hash3})
				handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
					Expect(reference).To(Equal(ref3))
					return nil
				}
				err = handler.DoAuth(ur, "", true)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".HandleReferences", func() {
		When("old state is not set", func() {
			BeforeEach(func() {
				err = handler.HandleReferences()
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("expected old state to have been captured"))
			})
		})

		When("reference handler function returns an error", func() {
			var err error
			BeforeEach(func() {
				handler.OldState = plumbing2.GetRepoState(repo)
				handler.ReferenceHandler = func(ref string) []error {
					return []error{fmt.Errorf("bad error")}
				}
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}

				err = handler.HandleReferences()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})

		When("it succeeds", func() {
			var err error
			BeforeEach(func() {
				handler.OldState = plumbing2.GetRepoState(repo)
				handler.ReferenceHandler = func(ref string) []error {
					return nil
				}
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {}}
				err = handler.HandleReferences()
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".HandleGCAndSizeCheck", func() {
		It("should return error when garbage collection execution failed", func() {
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GC("1 day ago").Return(fmt.Errorf("error"))
			handler.Repo = mockRepo
			err := handler.HandleGCAndSizeCheck()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to run garbage collection: error"))
		})

		It("should return error when unable to get repo size", func() {
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GC("1 day ago").Return(nil)
			mockRepo.EXPECT().Size().Return(float64(0), fmt.Errorf("error"))
			handler.Repo = mockRepo
			err := handler.HandleGCAndSizeCheck()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo size: error"))
		})

		It("should return error repo size exceeded limit", func() {
			params.MaxRepoSize = 999
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GC("1 day ago").Return(nil)
			mockRepo.EXPECT().Size().Return(float64(1000), nil)
			handler.Repo = mockRepo
			err := handler.HandleGCAndSizeCheck()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("size error: repository size has exceeded the network limit"))
		})

		It("should return error when unable to reload repo handle", func() {
			params.MaxRepoSize = 2000
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GC("1 day ago").Return(nil)
			mockRepo.EXPECT().Reload().Return(fmt.Errorf("error"))
			mockRepo.EXPECT().Size().Return(float64(1000), nil)
			handler.Repo = mockRepo
			err := handler.HandleGCAndSizeCheck()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to reload repo handle: error"))
		})

		It("should return no error", func() {
			params.MaxRepoSize = 2000
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GC("1 day ago").Return(nil)
			mockRepo.EXPECT().Reload().Return(nil)
			mockRepo.EXPECT().Size().Return(float64(1000), nil)
			handler.Repo = mockRepo
			err := handler.HandleGCAndSizeCheck()
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleReversion", func() {
		var errs []error

		When("there is only one reference in the push reader and reversion failed", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				handler.Reverter = func(repo types.LocalRepo, prevState types.RepoRefsState, options ...types.KVOption) (changes *types.Changes, err error) {
					return nil, fmt.Errorf("failed revert")
				}
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				errs = handler.HandleReversion()
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("refs/heads/master: failed to revert to old state: failed revert"))
			})
		})

		When("unable to get repo state", func() {
			BeforeEach(func() {
				mockRemoteSrv.EXPECT().GetRepoState(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
				handler.Server = mockRemoteSrv
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				handler.Reverter = func(repo types.LocalRepo, prevState types.RepoRefsState, options ...types.KVOption) (changes *types.Changes, err error) {
					return nil, fmt.Errorf("failed revert")
				}
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				errs = handler.HandleReversion()
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("refs/heads/master: failed to get current state: error"))
			})
		})

		When("there are two references in the push reader and reversion failed for both", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{
					"refs/heads/master": {NewHash: util.RandString(40)},
					"refs/heads/dev":    {NewHash: util.RandString(40)},
				}
				handler.Reverter = func(repo types.LocalRepo, prevState types.RepoRefsState, options ...types.KVOption) (changes *types.Changes, err error) {
					return nil, fmt.Errorf("failed revert")
				}
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				errs = handler.HandleReversion()
			})

			It("should return 2 errors", func() {
				Expect(errs).To(HaveLen(2))
				expected := Or(
					Equal("refs/heads/master: failed to revert to old state: failed revert"),
					Equal("refs/heads/dev: failed to revert to old state: failed revert"),
				)
				Expect(errs[0].Error()).To(expected)
				Expect(errs[1].Error()).To(expected)
			})
		})

		When("there is one reference in the push reader and reversion succeeded", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				errs = handler.HandleReversion()
			})

			It("should return no errors", func() {
				Expect(errs).To(HaveLen(0))
			})

			Specify("that repo state was reverted to old state", func() {
				curState := plumbing2.GetRepoState(repo)
				Expect(curState).To(Equal(handler.OldState))
			})
		})

		It("should perform revert operation when HandleReversion has been previously and successfully executed", func() {
			revertCount := 0
			handler.Server = svr
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
			remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
			handler.OldState = plumbing2.GetRepoState(repo)
			remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
			handler.Reverter = func(repo types.LocalRepo, prevState types.RepoRefsState, options ...types.KVOption) (*types.Changes, error) {
				revertCount++
				return nil, nil
			}
			errs = handler.HandleReversion()
			Expect(errs).To(HaveLen(0))
			errs = handler.HandleReversion()
			Expect(errs).To(HaveLen(0))
			Expect(revertCount).To(Equal(1))
		})

		It("should perform revert operation on multiple HandleReversion if previous call did not succeed", func() {
			revertCount := 0
			handler.Server = svr
			handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
			remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
			handler.OldState = plumbing2.GetRepoState(repo)
			remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
			handler.Reverter = func(repo types.LocalRepo, prevState types.RepoRefsState, options ...types.KVOption) (*types.Changes, error) {
				revertCount++
				return nil, fmt.Errorf("error")
			}
			errs = handler.HandleReversion()
			Expect(errs).To(HaveLen(1))
			errs = handler.HandleReversion()
			Expect(errs).To(HaveLen(1))
			Expect(revertCount).To(Equal(2))
		})
	})

	Describe(".HandlePushNote", func() {
		It("should return error when unable to add note to push pool", func() {
			note := &pushtypes.Note{}
			mockPushPool.EXPECT().Add(note).Return(fmt.Errorf("error"))
			err := handler.HandlePushNote(note)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should announce repo name and only broadcast note/endorsement if no error", func() {
			note := &pushtypes.Note{}
			mockPushPool.EXPECT().Add(note).Return(nil)

			mockSess := mocks.NewMockSession(ctrl)
			mockRemoteSrv.EXPECT().GetDHT().Return(mockDHT)
			mockDHT.EXPECT().NewAnnouncerSession().Return(mockSess)
			mockSess.EXPECT().Announce(announcer.ObjTypeRepoName, handler.Repo.GetName(), []byte(handler.Repo.GetName()))
			mockSess.EXPECT().OnDone(gomock.Any()).Do(func(cb func(errCount int)) {
				cb(0)
			})

			mockRemoteSrv.EXPECT().BroadcastNoteAndEndorsement(note)
			handler.HandlePushNote(note)
			time.Sleep(1 * time.Millisecond)
		})

		It("should not broadcast note/endorsement if announcement failed", func() {
			note := &pushtypes.Note{}
			mockPushPool.EXPECT().Add(note).Return(nil)

			mockSess := mocks.NewMockSession(ctrl)
			mockRemoteSrv.EXPECT().GetDHT().Return(mockDHT)
			mockDHT.EXPECT().NewAnnouncerSession().Return(mockSess)
			mockSess.EXPECT().Announce(announcer.ObjTypeRepoName, handler.Repo.GetName(), []byte(handler.Repo.GetName()))
			mockSess.EXPECT().OnDone(gomock.Any()).Do(func(cb func(errCount int)) {
				cb(1)
			})

			mockRemoteSrv.EXPECT().BroadcastNoteAndEndorsement(note).Times(0)
			handler.HandlePushNote(note)
			time.Sleep(1 * time.Millisecond)
		})

		It("should announce commit and tag objects only", func() {
			note := &pushtypes.Note{}
			mockPushPool.EXPECT().Add(note).Return(nil)

			commitObject := &push.PackObject{Type: plumbing.CommitObject, Hash: plumbing2.BytesToHash(util.RandBytes(20))}
			tagObject := &push.PackObject{Type: plumbing.TagObject, Hash: plumbing2.BytesToHash(util.RandBytes(20))}
			blobObject := &push.PackObject{Type: plumbing.BlobObject, Hash: plumbing2.BytesToHash(util.RandBytes(20))}
			handler.PushReader.Objects = []*push.PackObject{commitObject, tagObject, blobObject}

			mockSess := mocks.NewMockSession(ctrl)
			mockRemoteSrv.EXPECT().GetDHT().Return(mockDHT)
			mockDHT.EXPECT().NewAnnouncerSession().Return(mockSess)
			mockSess.EXPECT().Announce(announcer.ObjTypeRepoName, handler.Repo.GetName(), []byte(handler.Repo.GetName()))
			mockSess.EXPECT().Announce(announcer.ObjTypeGit, handler.Repo.GetName(), commitObject.Hash[:])
			mockSess.EXPECT().Announce(announcer.ObjTypeGit, handler.Repo.GetName(), tagObject.Hash[:])
			mockSess.EXPECT().OnDone(gomock.Any()).Do(func(cb func(errCount int)) {
				cb(0)
			})

			mockRemoteSrv.EXPECT().BroadcastNoteAndEndorsement(note)
			handler.HandlePushNote(note)
			time.Sleep(1 * time.Millisecond)
		})
	})

	Describe(".HandleRefMismatch", func() {
		It("should return error when unable to schedule resync", func() {
			handler.OldState = plumbing2.GetRepoState(repo)
			note := &pushtypes.Note{}
			mockRemoteSrv.EXPECT().TryScheduleReSync(note, "refs/heads/master", false).Return(fmt.Errorf("error"))
			err := handler.HandleRefMismatch(note, "refs/heads/master", false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when resync was scheduled successfully", func() {
			handler.OldState = plumbing2.GetRepoState(repo)
			note := &pushtypes.Note{}
			mockRemoteSrv.EXPECT().TryScheduleReSync(note, "refs/heads/master", false).Return(nil)
			err := handler.HandleRefMismatch(note, "refs/heads/master", false)
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleReference", func() {
		var errs []error

		Describe("when unable to get state of the repository", func() {
			BeforeEach(func() {
				handler.OldState = plumbing2.GetRepoState(repo)
				mockRemoteSrv.EXPECT().GetRepoState(repo, plumbing2.MatchOpt("refs/heads/master")).Return(nil, fmt.Errorf("error"))
				errs = handler.HandleReference("refs/heads/master")
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("failed to get current state: error"))
			})
		})

		Describe("when a reference failed change validation check", func() {
			var curState types.RepoRefsState
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("bad reference change")
				}

				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				curState = plumbing2.GetRepoState(repo)
				errs = handler.HandleReference("refs/heads/master")
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
			})

			It("should not revert the reference back to the old state", func() {
				newState := plumbing2.GetRepoState(repo)
				Expect(newState).To(Equal(curState))
			})
		})

		When("reference did not change", func() {
			var validated bool

			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					validated = true
					return nil
				}
				ref := "refs/heads/master"
				handler.TxDetails = map[string]*types.TxDetail{ref: {}}
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				handler.HandleReference("refs/heads/master")
			})

			It("should not validate the reference", func() {
				Expect(validated).To(BeFalse())
			})
		})

		Describe("when a reference is a post reference and update policy is required", func() {
			var policyChecked bool
			BeforeEach(func() {
				ref := plumbing2.MakeIssueReference(1)

				handler.Server = svr
				handler.Repo.SetState(state.BareRepository())
				handler.TxDetails = map[string]*types.TxDetail{ref: {FlagCheckAdminUpdatePolicy: true}}
				handler.PushReader.References = map[string]*push.PackedReferenceObject{ref: {NewHash: util.RandString(40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				}

				handler.PushReader.SetUpdateRequest(&packp.ReferenceUpdateRequest{Commands: []*packp.Command{{Name: plumbing.ReferenceName(ref)}}})
				handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContributor bool, action string) error {
					policyChecked = true
					return nil
				}

				remotetestutil.CreateCheckoutOrphanBranch(path, plumbing.ReferenceName(ref).Short())
				remotetestutil.AppendCommit(path, "body", "hello world", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "body", "hello world, again", "commit 1")
				errs = handler.HandleReference(ref)
				Expect(errs).To(HaveLen(0))
			})

			Specify("that policy was enforced", func() {
				Expect(policyChecked).To(BeTrue())
			})
		})

		Describe("when a reference is a post reference and update policy is required and policy enforcement is failed", func() {
			var policyChecked bool
			BeforeEach(func() {
				ref := plumbing2.MakeIssueReference(1)

				handler.Server = svr
				handler.Repo.SetState(state.BareRepository())
				handler.TxDetails = map[string]*types.TxDetail{ref: {FlagCheckAdminUpdatePolicy: true}}
				handler.PushReader.References = map[string]*push.PackedReferenceObject{ref: {NewHash: util.RandString(40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				}

				handler.PushReader.SetUpdateRequest(&packp.ReferenceUpdateRequest{Commands: []*packp.Command{{Name: plumbing.ReferenceName(ref)}}})
				handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContributor bool, action string) error {
					policyChecked = true
					return fmt.Errorf("policy failed")
				}

				remotetestutil.CreateCheckoutOrphanBranch(path, plumbing.ReferenceName(ref).Short())
				remotetestutil.AppendCommit(path, "body", "hello world", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "body", "hello world, again", "commit 1")
				errs = handler.HandleReference(ref)
			})

			Specify("that policy was enforced", func() {
				Expect(policyChecked).To(BeTrue())
			})

			Specify("that policy was enforced", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("authorization: policy failed"))
			})
		})

		Describe("when merge id is set in transaction info but merge compliance failed", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.TxDetails = map[string]*types.TxDetail{"refs/heads/master": {MergeProposalID: "001"}}
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: strings.Repeat("0", 40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				}
				handler.MergeChecker = func(repo types.LocalRepo, change *types.ItemChange, mergeProposalID, pushKeyID string, keepers core.Logic) error {
					return fmt.Errorf("failed merge check")
				}

				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				remotetestutil.AppendCommit(path, "file.txt", "line 1", "commit 2")
				errs = handler.HandleReference("refs/heads/master")
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): failed merge check"))
			})
		})
	})

	Describe(".HandleAnnouncement", func() {
		It("should announce repo name, commit and tag objects", func() {
			handler.Server = mockRemoteSrv
			c1Hash := plumbing.NewHash("49b3d65702d8dec55a7afa91513e80dcec82707b")
			t1Hash := plumbing.NewHash("db54d1823c36611a4450086fbdf07e5ff29036bb")
			b1Hash := plumbing.NewHash("1cd2897a8322731901acd4545bd1d81ab666e316")
			handler.PushReader.Objects = append(handler.PushReader.Objects,
				&push.PackObject{Type: plumbing.CommitObject, Hash: c1Hash},
				&push.PackObject{Type: plumbing.TagObject, Hash: t1Hash},
				&push.PackObject{Type: plumbing.BlobObject, Hash: b1Hash},
			)
			mockRemoteSrv.EXPECT().GetDHT().Return(mockDHT)
			mockSess := mocks.NewMockSession(ctrl)
			mockDHT.EXPECT().NewAnnouncerSession().Return(mockSess)
			mockSess.EXPECT().Announce(announcer.ObjTypeRepoName, handler.Repo.GetName(), []byte(handler.Repo.GetName()))
			mockSess.EXPECT().Announce(announcer.ObjTypeGit, handler.Repo.GetName(), c1Hash[:])
			mockSess.EXPECT().Announce(announcer.ObjTypeGit, handler.Repo.GetName(), t1Hash[:])
			mockSess.EXPECT().OnDone(gomock.Any()).Do(func(cb func(errCount int)) {
				cb(0)
			})

			var cbCalled bool
			handler.HandleAnnouncement(func(errCount int) {
				cbCalled = true
			})
			time.Sleep(1 * time.Millisecond)

			Expect(cbCalled).To(BeTrue())
		})
	})
})
