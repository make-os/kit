package mergecmd_test

import (
	"bytes"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/mergecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeReqStatus", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MergeReqStatusCmd", func() {
		It("should return error when unable to get reference", func() {
			ref := plumbing.MakeMergeRequestReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
			err := mergecmd.MergeReqStatusCmd(mockRepo, &mergecmd.MergeReqStatusArgs{Reference: ref})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when merge request reference does not exist", func() {
			ref := plumbing.MakeMergeRequestReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", plumbing.ErrRefNotFound)
			err := mergecmd.MergeReqStatusCmd(mockRepo, &mergecmd.MergeReqStatusArgs{Reference: ref})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("merge request not found"))
		})

		It("should return error when unable to read recent commit post body", func() {
			ref := plumbing.MakeMergeRequestReference(1)
			hash := "e31992a88829f3cb70ab5f5e964597a6c8f17047"
			mockRepo.EXPECT().RefGet(ref).Return(hash, nil)
			err := mergecmd.MergeReqStatusCmd(mockRepo, &mergecmd.MergeReqStatusArgs{
				Reference: ref,
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					return nil, nil, fmt.Errorf("error")
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read recent comment: error"))
		})

		It("should print 'open' when post body includes a closed=false directive", func() {
			ref := plumbing.MakeMergeRequestReference(1)
			hash := "e31992a88829f3cb70ab5f5e964597a6c8f17047"
			mockRepo.EXPECT().RefGet(ref).Return(hash, nil)
			buf := bytes.NewBuffer(nil)
			err := mergecmd.MergeReqStatusCmd(mockRepo, &mergecmd.MergeReqStatusArgs{
				Reference: ref,
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					closed := false
					return &plumbing.PostBody{Close: &closed}, nil, nil
				},
				StdOut: buf,
			})
			Expect(err).To(BeNil())
			Expect(buf.String()).To(Equal("open\n"))
		})

		It("should print 'closed' when post body includes a closed=true directive", func() {
			ref := plumbing.MakeMergeRequestReference(1)
			hash := "e31992a88829f3cb70ab5f5e964597a6c8f17047"
			mockRepo.EXPECT().RefGet(ref).Return(hash, nil)
			buf := bytes.NewBuffer(nil)
			err := mergecmd.MergeReqStatusCmd(mockRepo, &mergecmd.MergeReqStatusArgs{
				Reference: ref,
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					closed := true
					return &plumbing.PostBody{Close: &closed}, nil, nil
				},
				StdOut: buf,
			})
			Expect(err).To(BeNil())
			Expect(buf.String()).To(Equal("closed\n"))
		})
	})
})
