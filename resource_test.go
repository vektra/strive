package strive

import (
	"testing"

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
}
