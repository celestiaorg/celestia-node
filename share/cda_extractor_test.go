package share

import (
	"testing"

	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/stretchr/testify/require"
)

func TestSolveLinearSystem_Identity(t *testing.T) {
	var one blsfr.Element
	one.SetOne()

	G := [][]blsfr.Element{
		{one, blsfr.Element{}},
		{blsfr.Element{}, one},
	}

	var y0, y1 blsfr.Element
	y0.SetUint64(7)
	y1.SetUint64(9)

	x, err := SolveLinearSystem(G, []blsfr.Element{y0, y1})
	require.NoError(t, err)
	require.Equal(t, y0, x[0])
	require.Equal(t, y1, x[1])
}

func TestSolveLinearSystem_NonSingular(t *testing.T) {
	// Solve:
	// [2 3] [x0] = [5]
	// [1 4] [x1]   [6]
	//
	// Over a field, expected solution is:
	// x0 = (5*4 - 3*6) / (2*4 - 3*1) = (20 - 18) / (8 - 3) = 2/5
	// x1 = (2*6 - 5*1) / (2*4 - 3*1) = (12 - 5) / 5 = 7/5
	var a, b, c, d blsfr.Element
	a.SetUint64(2)
	b.SetUint64(3)
	c.SetUint64(1)
	d.SetUint64(4)

	G := [][]blsfr.Element{
		{a, b},
		{c, d},
	}

	var y0, y1 blsfr.Element
	y0.SetUint64(5)
	y1.SetUint64(6)

	x, err := SolveLinearSystem(G, []blsfr.Element{y0, y1})
	require.NoError(t, err)

	// Verify G*x == y.
	var lhs0, lhs1, tmp blsfr.Element
	lhs0.Mul(&a, &x[0])
	tmp.Mul(&b, &x[1])
	lhs0.Add(&lhs0, &tmp)

	lhs1.Mul(&c, &x[0])
	tmp.Mul(&d, &x[1])
	lhs1.Add(&lhs1, &tmp)

	require.Equal(t, y0, lhs0)
	require.Equal(t, y1, lhs1)
}

func TestSolveLinearSystem_Singular(t *testing.T) {
	// Rows are linearly dependent: second row = 2 * first row.
	var one, two blsfr.Element
	one.SetOne()
	two.SetUint64(2)

	G := [][]blsfr.Element{
		{one, one},
		{two, two},
	}

	var y0, y1 blsfr.Element
	y0.SetUint64(3)
	y1.SetUint64(6)

	_, err := SolveLinearSystem(G, []blsfr.Element{y0, y1})
	require.ErrorIs(t, err, ErrCDASingularMatrix)
}

