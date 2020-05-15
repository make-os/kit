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
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var commit *mocks.MockCommit

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		commit = mocks.NewMockCommit(ctrl)
		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ValidateIssueCommit", func() {
		var detail *core.TxDetail
		var args *validation.ValidateIssueCommitArg

		BeforeEach(func() {
			detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
			args = &validation.ValidateIssueCommitArg{OldHash: "", TxDetail: detail}
		})

		It("should return error when unable to check for merge commits", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			detail := &core.TxDetail{Reference: "refs/heads/issues/1"}
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change, TxDetail: detail}
			mockRepo.EXPECT().HasMergeCommits(detail.Reference).Return(false, fmt.Errorf("error"))
			err := validation.ValidateIssueCommit(mockRepo, nil, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check for merge commits in issue: error"))
		})

		It("should return error when issue has merge commits", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
			args = &validation.ValidateIssueCommitArg{OldHash: "", Change: change, TxDetail: detail}
			commit := repo.WrapCommit(&object.Commit{})
			mockRepo.EXPECT().HasMergeCommits(detail.Reference).Return(true, nil)
			err := validation.ValidateIssueCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue history must not include merge commits"))
		})

		It("should return error when commit failed commit validation", func() {
			commit := repo.WrapCommit(&object.Commit{})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change, TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("check error")
				},
			}
			err := validation.ValidateIssueCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("check error"))
		})

		It("should return error when unable to get ancestors", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			detail := &core.TxDetail{
				Reference: "refs/heads/issues/1",
			}

			mockRepo.EXPECT().GetAncestors(commit.UnWrap(), args.OldHash, true).Return(nil, fmt.Errorf("ancestor get error"))
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
				TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				},
				CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
					return nil, nil
				},
			}
			err := validation.ValidateIssueCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("ancestor get error"))
		})

		It("should return error when commit failed issue commit validation ", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			commit := repo.WrapCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
			detail := &core.TxDetail{Reference: "refs/heads/issues/1"}
			mockRepo.EXPECT().GetAncestors(commit.UnWrap(), args.OldHash, true).Return([]*object.Commit{}, nil)
			mockRepoState := state.BareRepository()
			mockRepo.EXPECT().GetState().Return(mockRepoState)

			callCount := 0
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
				TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				},
				CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
					callCount++
					Expect(commit).To(Equal(commit))
					Expect(args.Reference).To(Equal(detail.Reference))
					return nil, fmt.Errorf("issue check error")
				},
			}
			err := validation.ValidateIssueCommit(mockRepo, commit, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue check error"))
			Expect(callCount).To(Equal(1))
		})

		When("commit has no ancestor", func() {
			var commitObj *object.Commit
			var mockRepoState *state.Repository

			BeforeEach(func() {
				commitObj = &object.Commit{}
				commit.EXPECT().UnWrap().Return(commitObj)
				mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
				mockRepo.EXPECT().GetAncestors(commitObj, args.OldHash, true).Return([]*object.Commit{}, nil)
				mockRepoState = state.BareRepository()
				mockRepo.EXPECT().GetState().Return(mockRepoState)
			})

			It("should return no error when issue commit check is passed and"+
				"issue checker func must be called once", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: &core.TxDetail{Reference: "refs/heads/issues/1"},
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
			})

			It("should set tx detail FlagCheckIssueUpdatePolicy to true if issue body updates admin fields like 'labels'", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{Labels: &[]string{"label_update"}}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
				Expect(detail.FlagCheckIssueUpdatePolicy).To(BeTrue())
			})

			It("should populate tx detail reference data fields from issue body", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						cls := true
						return &plumbing2.IssueBody{
							Close:     &cls,
							Labels:    &[]string{"l1", "l2"},
							Assignees: &[]string{"key1", "key2"},
						}, nil
					},
				}

				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(1))
				Expect(*detail.Data().Close).To(Equal(true))
				Expect(*detail.Data().Labels).To(Equal([]string{"l1", "l2"}))
				Expect(*detail.Data().Assignees).To(Equal([]string{"key1", "key2"}))
			})

			It("should return error when issue reference has been previously closed and new issue commit did not set close=2", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{
					Hash:      []byte("hash"),
					IssueData: &state.IssueReferenceData{Closed: true},
				}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}

				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrCannotWriteToClosedIssue))
			})

			It("should return no error when issue reference has been previously closed and new issue commit set close=2", func() {
				commitObj.Hash = plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				detail = &core.TxDetail{Reference: "refs/heads/issues/1"}
				mockRepoState.References[detail.Reference] = &state.Reference{IssueData: &state.IssueReferenceData{Closed: true}, Hash: []byte("hash")}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++
						Expect(commit).To(Equal(commit))
						Expect(args.Reference).To(Equal(detail.Reference))
						cls := false
						return &plumbing2.IssueBody{Close: &cls}, nil
					},
				}

				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
			})
		})

		When("commit has an ancestor", func() {
			var child, ancestor *object.Commit

			BeforeEach(func() {
				child = &object.Commit{Hash: plumbing.NewHash("069199ae527ca118368d93af02feefa80432e563")}
				commit.EXPECT().UnWrap().Return(child)
				mockRepo.EXPECT().HasMergeCommits(gomock.Any()).Return(false, nil)
				ancestor = &object.Commit{Hash: plumbing.NewHash("c045fafe22ae2ef4d7c2390704e9b7a73c12bd43")}
				mockRepo.EXPECT().GetAncestors(child, args.OldHash, true).Return([]*object.Commit{ancestor}, nil)
				mockRepoState := state.BareRepository()
				mockRepo.EXPECT().GetState().Return(mockRepoState).Times(1)
			})

			Specify("that issue checker is called twice for both the commit and its ancestor", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: child.Hash.String()}}
				callCount := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: &core.TxDetail{Reference: "refs/heads/issues/1"},
					CheckCommit: func(commit *object.Commit, txDetail *core.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(r core.LocalRepo, commit core.Commit, args *validation.CheckIssueCommitArgs) (*plumbing2.IssueBody, error) {
						callCount++

						if callCount == 1 {
							Expect(commit.UnWrap()).To(Equal(ancestor))
							Expect(args.IsNewIssue).To(BeTrue())
						} else {
							Expect(commit.UnWrap()).To(Equal(child))
							Expect(args.IsNewIssue).To(BeFalse())
						}

						Expect(args.Reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, commit, args)
				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(2))
			})
		})
	})

	Describe(".CheckIssueCommit", func() {
		It("should return error when issue number is not valid", func() {
			args := &validation.CheckIssueCommitArgs{Reference: "refs/heads/" + plumbing2.IssueBranchPrefix + "/abc"}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue number is not valid. Must be numeric"))
		})

		It("should return error when issue commit has more than 1 parents", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(2)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit cannot have more than one parent"))
		})

		It("should return error when the reference of the issue commit is new but the issue commit has multiple parents ", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1).Times(2)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch, IsNewIssue: true}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("first commit of a new issue must have no parent"))
		})

		It("should return error when the issue commit alters history", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit must not alter history"))
		})

		It("should return error when unable to get commit tree", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			commit.EXPECT().GetTree().Return(nil, fmt.Errorf("bad query"))
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("unable to read issue commit tree"))
		})

		It("should return error when issue commit tree does not have 'body' file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit must have a 'body' file"))
		})

		It("should return error when issue commit tree has more than 1 files", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{}, {}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit tree must only include a 'body' file"))
		})

		It("should return error when issue commit tree has a body entry that isn't a regular file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{Name: "body", Mode: filemode.Dir}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			args := &validation.CheckIssueCommitArgs{Reference: issueBranch}
			_, err := validation.CheckIssueCommit(mockRepo, commit, args)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue body file is not a regular file"))
		})
	})

	Describe(".CheckIssueBody", func() {
		var commit *object.Commit

		BeforeEach(func() {
			commit = &object.Commit{Hash: plumbing2.MakeCommitHash("hash")}
		})

		It("should return error when an unexpected field exists", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.field1, msg:unknown field"))
		})

		It("should return error when an 'title' value is not string", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"title": 1}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:expected a string value"))
		})

		It("should return error when an 'replyTo' value is not string", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"replyTo": 1}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:expected a string value"))
		})

		It("should return error when a 'labels' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"labels": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a list of string values"))
		})

		It("should return error when a 'labels' are not valid", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true,
				map[string]interface{}{
					"title":  "title here",
					"labels": []interface{}{"bad_*la%bel"},
				}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#5efba81>.labels, msg:invalid characters in " +
				"identifier. Only alphanumeric, _, and - chars are allowed, " +
				"but _, - cannot be first chars"))
		})

		It("should return error when an 'assignees' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"assignees": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a list of string values"))
		})

		It("should return error when a 'reactions' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"reactions": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:expected a list of string values"))
		})

		It("should return error when a 'replyTo' field is set and issue commit is new", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"replyTo": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:not expected in a new issue commit"))
		})

		It("should return error when issue is not new, a 'replyTo' field is set and title is set", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), false, map[string]interface{}{"replyTo": "xyz", "title": "abc"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is not required when replying"))
		})

		It("should return error when issue is new and title is not set", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is required"))
		})

		It("should return error when issue is not new and title is set", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), false, map[string]interface{}{"title": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is not required for comment commit"))
		})

		It("should return error when title length is greater than max.", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"title": util.RandString(validation.MaxIssueTitleLen + 1)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is too long and cannot exceed 256 characters"))
		})

		It("should return error when replyTo value has length < 4 or > 40", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), false, map[string]interface{}{"replyTo": "abc"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))

			err = validation.CheckIssueBody(nil, repo.WrapCommit(commit), false, map[string]interface{}{"replyTo": util.RandString(41)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))
		})

		It("should return error when replyTo hash is not an ancestor", func() {
			replyTo := plumbing2.MakeCommitHash("hash").String()
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().IsAncestor(commit.Hash.String(), replyTo).Return(fmt.Errorf("error"))
			err := validation.CheckIssueBody(mockRepo, repo.WrapCommit(commit), false, map[string]interface{}{"replyTo": replyTo}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:not a valid hash of a commit in the issue"))
		})

		It("should return error when reactions field is not slice of string", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{1, 2, 3},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:expected a string list"))
		})

		It("should return error when reactions exceed max", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:too many reactions. Cannot exceed 10"))
		})

		It("should return error when a reaction is unknown", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{"unknown"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:unknown reaction"))
		})

		It("should return error when labels exceed max", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"labels": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:too many labels. Cannot exceed 10"))
		})

		It("should return error when labels does not include string values", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"labels": []interface{}{1, 2},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a string list"))
		})

		It("should return error when assignees does not include string values", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"assignees": []interface{}{1, 2},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a string list"))
		})

		It("should return error when assignees exceed max", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"assignees": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:too many assignees. Cannot exceed 10"))
		})

		It("should return error when assignees includes invalid push keys", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"assignees": []interface{}{"invalid_push_key"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.assignees, msg:invalid push key ID"))
		})

		It("should return error when issue is new but content is unset", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:issue content is required"))
		})

		It("should return error when content surpassed max. limit", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, util.RandBytes(validation.MaxIssueContentLen+1))
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:issue content length exceeded max character limit"))
		})

		It("should return no error when fields are acceptable", func() {
			err := validation.CheckIssueBody(nil, repo.WrapCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, []byte("abc"))
			Expect(err).To(BeNil())
		})
	})
})
