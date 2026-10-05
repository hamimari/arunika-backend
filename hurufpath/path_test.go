package hurufpath

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParityFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/svg_paths.json")
	require.NoError(t, err)
	var fx struct {
		Cases []struct {
			Path     string `json:"path"`
			Valid    bool   `json:"valid"`
			Segments int    `json:"segments"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &fx))
	require.NotEmpty(t, fx.Cases)
	for _, c := range fx.Cases {
		p, err := Parse(c.Path)
		if c.Valid {
			if assert.NoError(t, err, c.Path) {
				assert.Len(t, p.Segments, c.Segments, c.Path)
			}
		} else {
			assert.Error(t, err, c.Path)
		}
	}
}

func TestParsePoints(t *testing.T) {
	p, err := Parse("M200 80 C180 30 80 30 80 150")
	require.NoError(t, err)
	assert.Equal(t, Point{200, 80}, p.Start)
	assert.Equal(t, byte('C'), p.Segments[0].Cmd)
	assert.Equal(t, []Point{{180, 30}, {80, 30}, {80, 150}}, p.Segments[0].P)
}
