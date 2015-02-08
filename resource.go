package strive

import "errors"

type Resources map[*Resource]struct{}

func NewResources(slice []*Resource) Resources {
	ret := make(Resources)

	for _, res := range slice {
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
		return &Resource{
			Type:  r.Type,
			Value: NewIntValue(r.GetValue().GetIntVal() - o.GetValue().GetIntVal()),
		}, nil
	}

	return nil, ErrUnableToRemove
}
