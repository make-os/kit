package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/imroc/req"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Repo", func() {
	var ctrl *gomock.Controller
	var client *ClientV1
	var key *crypto.Key

	BeforeEach(func() {
		client = &ClientV1{apiRoot: ""}
		ctrl = gomock.NewController(GinkgoT())
		key = crypto.NewKeyFromIntSeed(1)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".CreateRepo", func() {
		It("should return error if signing key is not set", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, nil
			}
			_, err := client.CreateRepo(&types.CreateRepoBody{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signing key is required"))
		})

		It("should send payload and receive tx hash from server", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/repo/create"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("name"),
					HaveKey("config"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"address": "repo1", "hash": "0x12345"})
					w.WriteHeader(201)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.CreateRepo(&types.CreateRepoBody{
				Name:       "repo1",
				Nonce:      1,
				Value:      100,
				Fee:        1,
				Config:     state.DefaultRepoConfig.ToBasicMap(),
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal("repo1"))
			Expect(resp.Hash).To(Equal("0x12345"))
		})

		It("should return error if request fails", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, fmt.Errorf("error")
			}
			_, err := client.CreateRepo(&types.CreateRepoBody{SigningKey: key})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})

	Describe(".GetRepo", func() {
		It("should return error if request failed", func() {
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/repo/get"))
				Expect(params).To(HaveKey("name"))
				Expect(params).To(HaveKey("height"))
				Expect(params).To(HaveKey("noProposals"))
				Expect(params["name"]).To(Equal("repo1"))
				Expect(params["height"]).To(Equal(uint64(100)))
				Expect(params["noProposals"]).To(BeTrue())
				return resp, fmt.Errorf("error")
			}
			_, err := client.GetRepo("repo1", &types.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return repository on success", func() {
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/repo/get"))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					repo := state.BareRepository()
					repo.Balance = "100"
					data, _ := json.Marshal(repo)
					w.WriteHeader(200)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.GetRepo("repo1")
			Expect(err).To(BeNil())
			Expect(resp.Balance.String()).To(Equal("100"))
		})
	})

	Describe(".AddRepoContributors", func() {
		It("should return error if signing key is not set", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, nil
			}
			_, err := client.AddRepoContributors(&types.AddRepoContribsBody{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signing key is required"))
		})

		It("should send payload and receive tx hash from server", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/repo/addContributors"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("id"),
					HaveKey("name"),
					HaveKey("namespace"),
					HaveKey("namespaceOnly"),
					HaveKey("policies"),
					HaveKey("feeCap"),
					HaveKey("feeMode"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"address": "repo1", "hash": "0x12345"})
					w.WriteHeader(201)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.AddRepoContributors(&types.AddRepoContribsBody{
				RepoName:      "repo1",
				ProposalID:    "1",
				PushKeys:      []string{"push1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz"},
				FeeCap:        13.2,
				FeeMode:       12,
				Nonce:         1,
				Value:         10,
				Fee:           1.2,
				Namespace:     "ns1",
				NamespaceOnly: "ns1",
				Policies:      []*state.ContributorPolicy{{Object: "obj1", Action: "act1"}},
				SigningKey:    key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x12345"))
		})

		It("should return error if request fails", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, fmt.Errorf("error")
			}
			_, err := client.AddRepoContributors(&types.AddRepoContribsBody{SigningKey: key})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})

	Describe(".VoteRepoProposal", func() {
		It("should return error if signing key is not set", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, nil
			}
			_, err := client.VoteRepoProposal(&types.RepoVoteBody{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signing key is required"))
		})

		It("should send payload and receive tx hash from server", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/repo/vote"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("vote"),
					HaveKey("id"),
					HaveKey("name"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"address": "repo1", "hash": "0x12345"})
					w.WriteHeader(201)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.VoteRepoProposal(&types.RepoVoteBody{
				RepoName:   "repo1",
				ProposalID: "1",
				Nonce:      1,
				Fee:        1.2,
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x12345"))
		})

		It("should return error if request failed", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, fmt.Errorf("error")
			}
			_, err := client.VoteRepoProposal(&types.RepoVoteBody{SigningKey: key})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
