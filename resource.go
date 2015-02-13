package strive

import (
	"errors"

	"github.com/gogo/protobuf/proto"
)

type Resources map[*Resource]struct{}

func NewResources(slice []*Resource) Resources {
	ret := make(Resources)

	for _, res := range slice {
		ret.Add(res)
	}

	return ret
}

func (r Resources) Dup() Resources {
	ret := make(Resources)

	for res, _ := range r {
		ret.Add(res)
	}

	return ret
}

func (r Resources) Find(t Resource_ResourceType) (*Resource, bool) {
	for res, _ := range r {
		if res.GetType() == t {
			return res, true
		}
	}

	return nil, false
}

func (r Resources) FindAll(t Resource_ResourceType) []*Resource {
	var out []*Resource

	for res, _ := range r {
		if res.GetType() == t {
			out = append(out, res)
		}
	}

	return out
}

func (r Resources) Replace(orig, cur *Resource) {
	delete(r, orig)
	r[cur] = struct{}{}
}

func (r Resources) Add(res *Resource) {
	r[res] = struct{}{}
}

func (r Resources) Remove(res *Resource) {
	delete(r, res)
}

var ErrUnableToRemove = errors.New("unable to remove")

func (r *Resource) Remove(o *Resource) (*Resource, error) {
	if r.GetType() != o.GetType() {
		return nil, ErrUnableToRemove
	}

	switch r.GetType() {
	case Resource_CPU, Resource_MEMORY:
		if r.GetValue().GetIntVal() < o.GetValue().GetIntVal() {
			return nil, ErrUnableToRemove
		}

		return &Resource{
			Type:  r.Type,
			Value: NewIntValue(r.GetValue().GetIntVal() - o.GetValue().GetIntVal()),
		}, nil
	case Resource_PORT:
		nr, err := r.GetValue().GetRangeVal().Remove(o.GetValue().GetRangeVal())
		if err != nil {
			return nil, err
		}

		return &Resource{
			Type: r.Type,
			Value: &Value{
				ValueType: Value_RANGES.Enum(),
				RangeVal:  nr,
			},
		}, nil
	}

	return nil, ErrUnableToRemove
}

func (r *Range) Over(x *Range) bool {
	return x.GetStart() >= r.GetStart() && x.GetEnd() <= r.GetEnd()
}

func (r *Ranges) Remove(x *Ranges) (*Ranges, error) {
	var out []*Range

	for _, rr := range x.Ranges {
		var handled bool

		for _, cr := range r.Ranges {

			// If the current range doesn't extend completely over the remove
			// range, skip it.
			if !cr.Over(rr) {
				continue
			}

			handled = true

			switch {
			case rr.GetStart() == cr.GetStart():
				// rr == [1000, 1010], cr == [1000, 2000]
				if rr.GetEnd() < cr.GetEnd() {
					// out = [1011, 2000]
					out = append(out, &Range{
						Start: proto.Int64(rr.GetEnd() + 1),
						End:   cr.End,
					})
				}

			case rr.GetStart() > cr.GetStart():
				// rr == [1010, 2000], cr = [1000, 2000]
				if rr.GetEnd() == cr.GetEnd() {
					out = append(out, &Range{
						Start: proto.Int64(cr.GetStart()),
						End:   proto.Int64(rr.GetStart() - 1),
					})

					// rr == [1010, 1020], cr == [1000, 2000]
				} else {
					out = append(out, &Range{
						Start: proto.Int64(cr.GetStart()),
						End:   proto.Int64(rr.GetStart() - 1),
					})

					out = append(out, &Range{
						Start: proto.Int64(rr.GetEnd() + 1),
						End:   proto.Int64(cr.GetEnd()),
					})
				}
			}
		}

		// The remove range didn't fit within any current range
		if !handled {
			return nil, ErrUnableToRemove
		}
	}

	return &Ranges{Ranges: out}, nil
	/*

		a := r.Ranges[0]
		b := x.Ranges[0]

		return &Ranges{Ranges: []*Range{{Start: proto.Int64(*b.End + 1), End: a.End}}}
	*/
}
