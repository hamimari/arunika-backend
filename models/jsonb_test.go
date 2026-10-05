package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONB(t *testing.T) {
	var j JSONB
	v, err := j.Value()
	require.NoError(t, err)
	assert.Nil(t, v, "empty is NULL")

	require.NoError(t, j.Scan([]byte(`{"a":1}`)))
	v, _ = j.Value()
	assert.Equal(t, `{"a":1}`, v, "written as text so Postgres parses it as JSON")
	require.NoError(t, j.Scan(`[1]`))
	assert.Equal(t, `[1]`, string(j))
	require.NoError(t, j.Scan(nil))
	assert.Nil(t, j)
	assert.Error(t, j.Scan(42))

	out, err := json.Marshal(struct{ X JSONB }{JSONB(`{"b":true}`)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"X":{"b":true}}`, string(out))
	out, _ = json.Marshal(struct{ X JSONB }{})
	assert.JSONEq(t, `{"X":null}`, string(out))

	var in struct{ X JSONB }
	require.NoError(t, json.Unmarshal([]byte(`{"X":{"c":2}}`), &in))
	assert.Equal(t, `{"c":2}`, string(in.X))
	require.NoError(t, json.Unmarshal([]byte(`{"X":null}`), &in))
	assert.Nil(t, in.X)

	assert.Equal(t, `{"k":"v"}`, string(ToJSONB(map[string]string{"k": "v"})))
	assert.Nil(t, ToJSONB(nil))
	assert.Nil(t, ToJSONB(make(chan int)), "unmarshalable values are dropped")
}
