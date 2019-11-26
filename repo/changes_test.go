package repo

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Changes", func() {
	var err error
	var cfg *config.EngineConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".getChanges", func() {
		When("update is nil", func() {
			It("should return empty ref changes", func() {
				curState := &State{References: NewObjCol(map[string]Item{
					"ref": &Obj{Name: "abc"},
				})}
				changeLog := getChanges(curState.References, nil)
				Expect(changeLog).To(Equal(emptyChangeResult()))
			})
		})
	})

	Describe(".GetChanges - Check references", func() {

		When("update state is empty", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref": &Obj{Name: "abc"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return no ref changes", func() {
				Expect(changeLog.References.Changes).To(Equal([]*ItemChange{
					{
						Item:   &Obj{Type: "", Name: "abc", Data: ""},
						Action: ChangeTypeRemove,
					},
				}))
			})
		})

		When("update state is nil", func() {
			var curState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref": &Obj{Name: "abc"},
				})}
				changeLog = curState.GetChanges(nil)
			})

			It("should return no ref changes", func() {
				Expect(changeLog.References.Changes).To(BeEmpty())
			})
		})

		When("both current state and new state are empty", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{})}
				newState = &State{References: NewObjCol(map[string]Item{})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return no ref changes", func() {
				Expect(changeLog.References.Changes).To(BeEmpty())
			})
		})

		When("current state has 1 ref and new state has no refs", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{})}
				changeLog = curState.GetChanges(newState)
			})

			It("should set size change to true", func() {
				Expect(changeLog.References.SizeChange).To(BeTrue())
			})

			It("should return 1 ref change with action=remove", func() {
				Expect(changeLog.References.Changes).To(HaveLen(1))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref1", Data: "hash1"},
					Action: ChangeTypeRemove,
				}))
			})
		})

		When("current state has refs=[ref1] and new state has refs=[ref1]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should set size change to false", func() {
				Expect(changeLog.References.SizeChange).To(BeFalse())
			})

			It("should return no ref changes", func() {
				Expect(changeLog.References.Changes).To(HaveLen(0))
			})
		})

		When("current state has refs=[ref1,ref2] and new state has refs=[ref1]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
					"ref2": &Obj{Name: "ref2", Data: "hash2"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return 1 ref change with action=remove", func() {
				Expect(changeLog.References.Changes).To(HaveLen(1))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref2", Data: "hash2"},
					Action: ChangeTypeRemove,
				}))
			})
		})

		When("current state has refs=[ref1] and new state has refs=[ref1,ref2]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
					"ref2": &Obj{Name: "ref2", Data: "hash2"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return 1 ref change with action=add", func() {
				Expect(changeLog.References.Changes).To(HaveLen(1))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref2", Data: "hash2"},
					Action: ChangeTypeNew,
				}))
			})
		})

		When("current state has refs=[ref1] and new state has refs=[ref1,ref2,ref3]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
					"ref2": &Obj{Name: "ref2", Data: "hash2"},
					"ref3": &Obj{Name: "ref3", Data: "hash3"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return 2 ref changes [{ref2,add},{ref3,add}]", func() {
				Expect(changeLog.References.Changes).To(HaveLen(2))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref2", Data: "hash2"},
					Action: ChangeTypeNew,
				}))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref3", Data: "hash3"},
					Action: ChangeTypeNew,
				}))
			})
		})

		When("current state has refs=[ref1,ref2,ref3] and new state has refs=[ref1,ref4]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
					"ref2": &Obj{Name: "ref2", Data: "hash2"},
					"ref3": &Obj{Name: "ref3", Data: "hash3"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash2"},
					"ref4": &Obj{Name: "ref4", Data: "hash4"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return 2 ref changes [{ref1,update},{ref2,remove},{ref3,remove},{ref4,add}]", func() {
				Expect(changeLog.References.Changes).To(HaveLen(4))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref1", Data: "hash2"},
					Action: ChangeTypeUpdate,
				}))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref2", Data: "hash2"},
					Action: ChangeTypeRemove,
				}))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref3", Data: "hash3"},
					Action: ChangeTypeRemove,
				}))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref4", Data: "hash4"},
					Action: ChangeTypeNew,
				}))
			})
		})

		When("current state has refs=[{ref1,hash=hash1}] and new state has refs=[{ref1,hash=hash_x}]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash1"},
				})}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash_x"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should set size change to false", func() {
				Expect(changeLog.References.SizeChange).To(BeFalse())
			})

			It("should return 1 ref change with action=replace", func() {
				Expect(changeLog.References.Changes).To(HaveLen(1))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref1", Data: "hash_x"},
					Action: ChangeTypeUpdate,
				}))
			})
		})

		When("current state is empty and new state has refs=[{ref1,hash=hash_x}]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(nil)}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash_x"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return newState sole item as a ChangeTypeNew", func() {
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref1", Data: "hash_x"},
					Action: ChangeTypeNew,
				}))
			})
		})

		When("current state is empty and new state has refs=[{ref1,hash=hash_x},{ref2,hash=hash2_x}]", func() {
			var curState, newState *State
			var changeLog *Changes
			BeforeEach(func() {
				curState = &State{References: NewObjCol(nil)}
				newState = &State{References: NewObjCol(map[string]Item{
					"ref1": &Obj{Name: "ref1", Data: "hash_x"},
					"ref2": &Obj{Name: "ref2", Data: "hash2_x"},
				})}
				changeLog = curState.GetChanges(newState)
			})

			It("should return newState items as actions of type ChangeTypeNew", func() {
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref1", Data: "hash_x"},
					Action: ChangeTypeNew,
				}))
				Expect(changeLog.References.Changes).To(ContainElement(&ItemChange{
					Item:   &Obj{Name: "ref2", Data: "hash2_x"},
					Action: ChangeTypeNew,
				}))
			})
		})
	})
})

var _ = Describe("ObjCol", func() {
	Describe(".Has", func() {
		It("should return true if entry with name exist", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			Expect(refs.Has("ref1")).To(BeTrue())
		})
		It("should return false if entry with name does not exist", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			Expect(refs.Has("ref")).To(BeFalse())
		})
	})

	Describe(".Get", func() {
		It("should get ref when it exists", func() {
			r := &Obj{Name: "ref1"}
			refs := NewObjCol(map[string]Item{"ref1": r})
			res := refs.Get(r.Name)
			Expect(r).To(Equal(res))
		})

		It("should return nil when ref does not exists", func() {
			refs := NewObjCol(map[string]Item{})
			res := refs.Get("ref1")
			Expect(res).To(BeNil())
		})
	})

	Describe(".Len", func() {
		It("should return 0 when empty", func() {
			refs := NewObjCol(map[string]Item{})
			Expect(refs.Len()).To(Equal(int64(0)))
		})

		It("should return 1", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			Expect(refs.Len()).To(Equal(int64(1)))
		})
	})

	Describe(".ForEach", func() {
		It("should iterate through all items", func() {
			refs := NewObjCol(map[string]Item{
				"ref1": &Obj{Name: "ref1"},
				"ref2": &Obj{Name: "ref2"},
			})
			var seen []interface{}
			refs.ForEach(func(i Item) bool {
				seen = append(seen, i)
				return false
			})
			Expect(seen).To(HaveLen(2))
		})

		It("should break after the first item", func() {
			refs := NewObjCol(map[string]Item{
				"ref1": &Obj{Name: "ref1"},
				"ref2": &Obj{Name: "ref2"},
			})
			var seen []interface{}
			refs.ForEach(func(i Item) bool {
				seen = append(seen, i)
				return true
			})
			Expect(seen).To(HaveLen(1))
		})
	})

	Describe(".Equal", func() {
		It("should return true when equal", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			refs2 := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			Expect(refs.Equal(refs2)).To(BeTrue())
		})

		It("should return false when values differ (case 1)", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			refs2 := NewObjCol(map[string]Item{"ref2": &Obj{Name: "ref2"}})
			Expect(refs.Equal(refs2)).To(BeFalse())
		})

		It("should return false when values differ (case 2)", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1"}})
			refs2 := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref2"}})
			Expect(refs.Equal(refs2)).To(BeFalse())
		})

		It("should return false when values differ (case 3)", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1", Data: "abc"}})
			refs2 := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1", Data: "xyz"}})
			Expect(refs.Equal(refs2)).To(BeFalse())
		})

		It("should return false when length differ (case 4)", func() {
			refs := NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1", Data: "abc"}})
			refs2 := NewObjCol(map[string]Item{
				"ref1": &Obj{Name: "ref1", Data: "abc"},
				"ref2": &Obj{Name: "ref2", Data: "xyz"},
			})
			Expect(refs.Equal(refs2)).To(BeFalse())
		})
	})

	Describe(".Bytes", func() {
		var col *ObjCol
		var expected []byte
		BeforeEach(func() {
			col = NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1", Data: "abc"}})
			expected = []uint8{
				0x81, 0xa4, 0x72, 0x65, 0x66, 0x31, 0x83, 0xa4, 0x54, 0x79, 0x70, 0x65, 0xa0, 0xa4, 0x4e, 0x61,
				0x6d, 0x65, 0xa4, 0x72, 0x65, 0x66, 0x31, 0xa4, 0x44, 0x61, 0x74, 0x61, 0xa3, 0x61, 0x62, 0x63,
			}
		})

		It("should return bytes", func() {
			bz := col.Bytes()
			Expect(bz).To(Equal(expected))
		})
	})

	Describe(".Hash", func() {
		var col *ObjCol
		BeforeEach(func() {
			col = NewObjCol(map[string]Item{"ref1": &Obj{Name: "ref1", Data: "abc"}})
		})

		It("should return hash", func() {
			hash := col.Hash()
			Expect(hash).To(HaveLen(32))
			Expect(hash).To(Equal(util.BytesToHash([]byte{100, 185, 179, 176, 214, 78, 213, 195,
				180, 8, 68, 146, 117, 8, 171, 67, 82, 186, 38, 50, 150, 182, 22, 198, 127, 82,
				135, 70, 137, 36, 28, 33})))
		})
	})
	
})
