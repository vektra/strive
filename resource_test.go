package strive

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektra/neko"
)

func TestResource(t *testing.T) {
	n := neko.Start(t)

	n.It("can remove partial resources", func() {
		r1 := &Resource{
			Type:  Resource_CPU.Enum(),
			Value: NewIntValue(4),
		}

		r2 := &Resource{
			Type:  Resource_CPU.Enum(),
			Value: NewIntValue(1),
		}

		r3, err := r1.Remove(r2)
		require.NoError(t, err)

		assert.Equal(t, 3, r3.GetValue().GetIntVal())
	})

	n.Meow()
}

func TestRange(t *testing.T) {
	n := neko.Start(t)

	full := &Ranges{Ranges: []*Range{{Start: proto.Int64(1000), End: proto.Int64(2000)}}}

	n.It("returns a new range by subtracting one from another", func() {
		a := &Ranges{Ranges: []*Range{{Start: proto.Int64(1000), End: proto.Int64(1010)}}}

		out, err := full.Remove(a)
		require.NoError(t, err)

		x := out.Ranges[0]

		assert.Equal(t, 1011, x.GetStart())
		assert.Equal(t, 2000, x.GetEnd())
	})

	n.It("splits a range in 2", func() {
		a := &Ranges{Ranges: []*Range{{Start: proto.Int64(1010), End: proto.Int64(1020)}}}

		out, err := full.Remove(a)
		require.NoError(t, err)

		x := out.Ranges[0]

		assert.Equal(t, 1000, x.GetStart())
		assert.Equal(t, 1009, x.GetEnd())

		x = out.Ranges[1]

		assert.Equal(t, 1021, x.GetStart())
		assert.Equal(t, 2000, x.GetEnd())
	})

	n.It("subtracts a range at the end of another", func() {
		a := &Ranges{Ranges: []*Range{{Start: proto.Int64(1900), End: proto.Int64(2000)}}}

		out, err := full.Remove(a)
		require.NoError(t, err)

		x := out.Ranges[0]

		assert.Equal(t, 1000, x.GetStart())
		assert.Equal(t, 1899, x.GetEnd())
	})

	n.Meow()
}
