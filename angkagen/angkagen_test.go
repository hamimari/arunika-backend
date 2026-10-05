package angkagen

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite testdata/generator_golden.json")

const goldenPath = "testdata/generator_golden.json"

// goldenCase stores each question compactly as
// [count, object_id, size, layout, [x1, y1, x2, y2, …]].
type goldenCase struct {
	Params    Params            `json:"params"`
	Questions []json.RawMessage `json:"questions"`
}

func compact(qs []Question) []json.RawMessage {
	out := make([]json.RawMessage, len(qs))
	for i, q := range qs {
		flat := make([]int, 0, 2*len(q.Points))
		for _, p := range q.Points {
			flat = append(flat, p.X, p.Y)
		}
		out[i], _ = json.Marshal([]interface{}{q.Count, q.ObjectID, q.Size, q.Layout, flat})
	}
	return out
}

// goldenParams builds the fixture inputs from a fixed seed, forcing the edge
// shapes (R = 1, R > n, every size bucket, both layouts) into the first cases.
func goldenParams() []Params {
	r := NewRand(2026)
	objects := []string{"apel", "bola", "ikan", "bebek"}
	fixed := []Params{
		{Min: 3, Max: 3, Count: 5, Layout: LayoutScatter},   // R = 1
		{Min: 1, Max: 20, Count: 5, Layout: LayoutScatter},  // R > n
		{Min: 11, Max: 20, Count: 10, Layout: LayoutRows},   // level 3
		{Min: 1, Max: 5, Count: 10, Layout: LayoutScatter},  // level 1
		{Min: 1, Max: 10, Count: 10, Layout: LayoutScatter}, // level 2
		{Min: 16, Max: 20, Count: 20, Layout: LayoutScatter},
		{Min: 1, Max: 2, Count: 20, Layout: LayoutRows},
	}
	var out []Params
	for i := 0; i < 500; i++ {
		var p Params
		if i < len(fixed) {
			p = fixed[i]
		} else {
			lo := 1 + r.Intn(20)
			hi := lo + r.Intn(21-lo)
			p = Params{Min: lo, Max: hi, Count: 5 + r.Intn(16), Layout: LayoutScatter}
			if r.Intn(2) == 0 {
				p.Layout = LayoutRows
			}
		}
		p.ObjectIDs = objects[:1+i%len(objects)]
		p.Seed = r.Next()
		out = append(out, p)
	}
	return out
}

func TestGolden(t *testing.T) {
	params := goldenParams()
	if *update {
		cases := make([]goldenCase, len(params))
		for i, p := range params {
			cases[i] = goldenCase{Params: p, Questions: compact(Generate(p))}
		}
		b, err := json.Marshal(cases)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(goldenPath, b, 0o644))
	}
	b, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	var cases []goldenCase
	require.NoError(t, json.Unmarshal(b, &cases))
	require.GreaterOrEqual(t, len(cases), 500)
	for i, c := range cases {
		assert.Equal(t, c.Questions, compact(Generate(c.Params)), "case %d", i)
	}
}

func TestMulberry32KnownValues(t *testing.T) {
	// Reference values from the canonical JavaScript mulberry32.
	r := NewRand(1)
	assert.Equal(t, []uint32{2693262067, 11749833, 2265367787}, []uint32{r.Next(), r.Next(), r.Next()})
}

func TestProperties(t *testing.T) {
	r := NewRand(99)
	objects := []string{"a", "b", "c"}
	for i := 0; i < 10000; i++ {
		lo := 1 + r.Intn(20)
		hi := lo + r.Intn(21-lo)
		p := Params{
			Min: lo, Max: hi, Count: 5 + r.Intn(16),
			ObjectIDs: objects[:1+r.Intn(3)], Layout: LayoutScatter, Seed: r.Next(),
		}
		if r.Intn(2) == 0 {
			p.Layout = LayoutRows
		}
		qs := Generate(p)
		name := fmt.Sprintf("case %d %+v", i, p)
		require.Len(t, qs, p.Count, name)
		require.Equal(t, qs, Generate(p), name)

		span := hi - lo + 1
		seen := map[int]int{}
		for k, q := range qs {
			require.True(t, q.Count >= lo && q.Count <= hi, name)
			seen[q.Count]++
			if k > 0 && span > 1 {
				require.NotEqual(t, qs[k-1].Count, q.Count, name)
			}
			if k > 0 && len(p.ObjectIDs) > 1 {
				require.NotEqual(t, qs[k-1].ObjectID, q.ObjectID, name)
			}
			require.Len(t, q.Points, q.Count, name)
			min := q.Size + Gap
			for a, pa := range q.Points {
				require.True(t, pa.X-q.Size/2 >= 0 && pa.X+q.Size/2 <= AreaWidth, name)
				require.True(t, pa.Y-q.Size/2 >= 0 && pa.Y+q.Size/2 <= AreaHeight, name)
				for _, pb := range q.Points[a+1:] {
					dx, dy := pa.X-pb.X, pa.Y-pb.Y
					require.GreaterOrEqual(t, dx*dx+dy*dy, min*min, name)
				}
			}
		}
		if span <= p.Count {
			require.Len(t, seen, span, "every value covered: "+name)
		} else {
			require.Len(t, seen, p.Count, "distinct counts: "+name)
		}
	}
}

func TestRowsSplit(t *testing.T) {
	pts := rows(7, PictureSize(7))
	require.Len(t, pts, 7)
	assert.Equal(t, pts[0].Y, pts[3].Y)
	assert.NotEqual(t, pts[3].Y, pts[4].Y, "7 pictures split 4 + 3")
}
