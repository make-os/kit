package push_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/policy"
	"github.com/themakeos/lobe/remote/push"
	types2 "github.com/themakeos/lobe/remote/push/types"
	repo3 "github.com/themakeos/lobe/remote/repo"
	"github.com/themakeos/lobe/remote/server"
	testutil2 "github.com/themakeos/lobe/remote/testutil"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

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

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockLogic = mocks.NewMockLogic(ctrl)
		mockDHT = mocks.NewMockDHT(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		svr = server.NewRemoteServer(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)
		mockRemoteSrv = mocks.NewMockRemoteServer(ctrl)
		mockRemoteSrv.EXPECT().Log().Return(cfg.G().Log)

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
				err = handler.HandleStream(nil, nil)
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
				err = handler.HandleStream(strings.NewReader("invalid"), nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to decode request pack: invalid pkt-len found"))
			})
		})

		When("packfile is valid", func() {
			var packfile io.ReadSeeker

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				note := &types2.Note{
					TargetRepo: repo,
					References: []*types2.PushedReference{
						{Name: "refs/heads/master", NewHash: commitHash, OldHash: plumbing.ZeroHash.String()},
					},
				}
				packfile, err = push.MakeReferenceUpdateRequestPack(note)
				Expect(err).To(BeNil())
			})

			When("old state is unset", func() {
				It("should return error when unable to get repo state", func() {
					mockRemoteSrv.EXPECT().GetRepoState(handler.Repo).Return(nil, fmt.Errorf("error"))
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
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
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("authorization: auth failed badly"))
				})
			})

			When("authorization succeeds", func() {
				It("should return no error", func() {
					handler.OldState = &plumbing2.State{}
					handler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
						return nil
					}
					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).To(BeNil())
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
			"policy is 'PolicyActionIssueWrite'", func() {
			ref := plumbing2.MakeMergeRequestReference(1)
			handler.PushReader.References[ref] = &push.PackedReferenceObject{}
			handler.TxDetails[ref] = &types.TxDetail{MergeProposalID: "123"}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(ref), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(ref))
				Expect(action).To(Equal(policy.PolicyActionMergeRequestWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with non-zero new hash, policy is 'PolicyActionIssueWrite'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.PushReader.References[issueBranch] = &push.PackedReferenceObject{}
			handler.TxDetails[issueBranch] = &types.TxDetail{MergeProposalID: ""}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionIssueWrite))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with zero new hash, policy is 'PolicyActionIssueDelete'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.PushReader.References = map[string]*push.PackedReferenceObject{issueBranch: {}}
			handler.TxDetails[plumbing2.MakeIssueReference(1)] = &types.TxDetail{}
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: plumbing.ZeroHash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionIssueDelete))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that for issue reference with non-zero new hash and"+
			"the reference previously exist and"+
			"tx detail FlagCheckIssueUpdatePolicy is true, "+
			"policy is 'PolicyActionIssueUpdate'", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			handler.Repo.GetState().References[issueBranch] = &state.Reference{Hash: []byte("hash")}
			handler.PushReader.References = map[string]*push.PackedReferenceObject{issueBranch: {}}
			handler.TxDetails[issueBranch] = &types.TxDetail{FlagCheckAdminUpdatePolicy: true}
			hash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
			ur.Commands = append(ur.Commands, &packp.Command{Name: plumbing.ReferenceName(issueBranch), New: hash})
			handler.PolicyChecker = func(enforcer policy.EnforcerFunc, reference string, isRefCreator bool, pushKeyID string, isContrib bool, action string) error {
				Expect(reference).To(Equal(issueBranch))
				Expect(action).To(Equal(policy.PolicyActionIssueUpdate))
				return nil
			}
			err = handler.DoAuth(ur, "", false)
			Expect(err).To(BeNil())
		})

		Specify("that policy action is PolicyActionIssueWrite when command is "+
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
				Expect(action).To(Equal(policy.PolicyActionIssueWrite))
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
				Expect(action).To(Equal(policy.PolicyActionMergeRequestWrite))
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
				Expect(action).To(Equal(policy.PolicyActionMergeRequestDelete))
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
				Expect(action).To(Equal(policy.PolicyActionMergeRequestUpdate))
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
				handler.ReferenceHandler = func(ref string, revertOnly bool) []error {
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
	})

	Describe(".HandleReference", func() {
		var errs []error
		Describe("when unable to get state of the repository", func() {
			BeforeEach(func() {
				handler.OldState = plumbing2.GetRepoState(repo)
				mockRemoteSrv.EXPECT().GetRepoState(repo, plumbing2.MatchOpt("refs/heads/master")).Return(nil, fmt.Errorf("error"))
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("failed to get current state: error"))
			})
		})

		Describe("when a reference failed change validation check", func() {
			BeforeEach(func() {
				handler.Server = svr
				handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
				handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("bad reference change")
				}

				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
			})

			It("should revert the reference back to the old state", func() {
				newState := plumbing2.GetRepoState(repo)
				Expect(newState).To(Equal(handler.OldState))
			})
		})

		Context("check reversion", func() {
			Describe("when a reference failed change validation and reversion failed", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
					handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return fmt.Errorf("bad reference change")
					}
					handler.Reverter = func(repo types.LocalRepo, prevState types.BareRepoRefsState, options ...types.KVOption) (changes *types.Changes, err error) {
						return nil, fmt.Errorf("failed revert")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return 2 error", func() {
					Expect(errs).To(HaveLen(2))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
					Expect(errs[1].Error()).To(Equal("failed to revert to old state: failed revert"))
				})
			})

			Describe("when a reference failed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
					handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return fmt.Errorf("bad reference change")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return 1 error", func() {
					Expect(errs).To(HaveLen(1))
					Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): bad reference change"))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
				})
			})

			Describe("when a reference failed change validation but revertOnly is true", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
					handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return fmt.Errorf("bad reference change")
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", true)
				})

				It("should return no error since validation is skipped", func() {
					Expect(errs).To(HaveLen(0))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
				})
			})

			Describe("when a reference passed change validation and reversion succeeds", func() {
				BeforeEach(func() {
					handler.Server = svr
					handler.TxDetails = map[string]*types.TxDetail{"refs/heads/master": {}}
					handler.PushReader.References = map[string]*push.PackedReferenceObject{"refs/heads/master": {NewHash: util.RandString(40)}}
					handler.ChangeValidator = func(keepers core.Keepers, repo types.LocalRepo, oldHash string, change *types.ItemChange, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					}

					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
					errs = handler.HandleReference("refs/heads/master", false)
				})

				It("should return no error", func() {
					Expect(errs).To(HaveLen(0))
				})

				Specify("that repo state was reverted to old state", func() {
					curState := plumbing2.GetRepoState(repo)
					Expect(curState).To(Equal(handler.OldState))
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

					testutil2.CreateCheckoutOrphanBranch(path, plumbing.ReferenceName(ref).Short())
					testutil2.AppendCommit(path, "body", "hello world", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "body", "hello world, again", "commit 1")
					errs = handler.HandleReference(ref, false)
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

					testutil2.CreateCheckoutOrphanBranch(path, plumbing.ReferenceName(ref).Short())
					testutil2.AppendCommit(path, "body", "hello world", "commit 1")
					handler.OldState = plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "body", "hello world, again", "commit 1")
					errs = handler.HandleReference(ref, false)
				})

				Specify("that policy was enforced", func() {
					Expect(policyChecked).To(BeTrue())
				})

				Specify("that policy was enforced", func() {
					Expect(errs).To(HaveLen(1))
					Expect(errs[0].Error()).To(Equal("authorization: policy failed"))
				})
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

				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				handler.OldState = plumbing2.GetRepoState(repo)
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 2")
				errs = handler.HandleReference("refs/heads/master", false)
			})

			It("should return 1 error", func() {
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(Equal("validation error (refs/heads/master): failed merge check"))
			})
		})
	})
})
