package pulp

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
)

func (s StaticDynamic) Render() string {
	res := strings.Builder{}

	for i := range s.Static {
		res.WriteString(s.Static[i])

		if ok := i < len(s.Dynamic); ok {

			switch r := s.Dynamic[i].(type) { // TODO: remove this switch and instead check if the type implements RenderDiff and use that Render method if this is the case

			case For:
				res.WriteString(r.Render())

			case IfTemplate:
				ifStr := ""

				if *r.Condition {
					ifStr = r.True.String()
				} else {
					ifStr = r.False.String()
				}
				res.WriteString(ifStr)

			case ForTemplate:
				notreached()

				forStr := strings.Builder{}

				for _, dynamic := range r.Dynamics {
					fmt.Fprint(&forStr, StaticDynamic{Static: r.Static, Dynamic: dynamic}.String())
				}

				res.WriteString(forStr.String())
			default:
				res.WriteString(fmt.Sprint(s.Dynamic[i]))
			}
		}
	}

	return res.String()
}

func notreached() {
	panic("should not be reached")
}

// Patches can point to actual value itself or another layer of Patches
type Patches map[string]interface{}

func (p Patches) IsEmpty() bool {
	return len(map[string]interface{}(p)) == 0
}

type Diffable interface {
	Diff(new interface{}) *Patches
}

type Renderable interface {
	Render() string
}

type RenderDiff interface {
	Diffable
	Renderable
}

// Dynamics can be filled by actual values or itself by other Diffables
type Dynamics []interface{}

var _ = Dynamics{0, 1}

func (d Dynamics) Diff(new interface{}) *Patches {
	new_ := new.(Dynamics)

	if len(d) != len(new_) {
		panic(fmt.Errorf("expected equal length in Dynamics"))
	}

	ret := Patches{}

	for i := 0; i < len(d); i++ {
		if d1Diffable, isDiffable := d[i].(Diffable); isDiffable {
			if diff := d1Diffable.Diff(new_[i]); diff != nil {
				ret[fmt.Sprint(i)] = diff
			}
		} else {
			if !cmp.Equal(d[i], new_[i]) {
				ret[fmt.Sprint(i)] = new_[i]
			}
		}
	}

	if ret.IsEmpty() { // does this yield the length of keys in the map?
		return nil
	}

	return &ret
}

var _ Diffable = If{}

type If struct {
	Condition bool          `json:"c"`
	True      StaticDynamic `json:"t"`
	False     StaticDynamic `json:"f"`
}

func (old If) Diff(new interface{}) *Patches {
	new_ := new.(If)

	patches := Patches{}

	if old.Condition != new_.Condition {
		patches["c"] = new_.Condition
	}

	// if new_.Condition {
	if trueDiff := old.True.Dynamic.Diff(new_.True.Dynamic); trueDiff != nil {
		patches["t"] = trueDiff
	}
	// } else {
	if falseDiff := old.False.Dynamic.Diff(new_.False.Dynamic); falseDiff != nil {
		patches["f"] = falseDiff
	}
	// }

	if patches.IsEmpty() {
		return nil
	}

	return &patches
}

var _ RenderDiff = For{}

type For struct {
	Statics      []string   `json:"s"`
	ManyDynamics []Dynamics `json:"ds"`
	DiffStrategy `json:"strategy"`
}

type DiffStrategy uint8

const (
	Append DiffStrategy = iota
	Prepend
)

func (f For) Render() string {
	str := strings.Builder{}

	for _, dynamics := range f.ManyDynamics {
		str.WriteString(StaticDynamic{f.Statics, dynamics}.Render())
	}

	return str.String()
}

func (old For) Diff(new interface{}) *Patches {
	new_ := new.(For)

	patches := Patches{}

	for i, dynamics := range old.ManyDynamics {
		if diff := dynamics.Diff(new_.ManyDynamics[i]); diff != nil {
			patches[fmt.Sprint(i)] = diff // TODO: check here, if the dynamic value has a key and use that instead. default to the slower index approach seen here
		}
	}

	if hasNewElements := len(old.ManyDynamics) < len(new_.ManyDynamics); hasNewElements {
		base := len(old.ManyDynamics)
		newElements := new_.ManyDynamics[base:]
		for i, dynamics := range newElements {
			patches[fmt.Sprint(i+base)] = dynamics
		}
	}

	if patches.IsEmpty() {
		return nil
	}

	return &Patches{
		"ds": patches,
	}
}