// Package hurufpath parses Belajar Huruf tracing guide strokes.
//
// The grammar is deliberately small and shared with the app (Dart) and the
// backoffice (TypeScript): one open subpath of absolute M, L, Q and C
// commands on the 300 × 300 grid. testdata/svg_paths.json is the parity
// fixture all three parsers must agree on.
package hurufpath

import (
	"errors"
	"fmt"
	"strconv"
)

// Grid is the side of the tracing grid; coordinates must lie within 0..Grid.
const Grid = 300

// Point is a coordinate on the grid.
type Point struct{ X, Y float64 }

// Segment is one drawing command after the initial M: "L" (P[0]), "Q" (P[0]
// control, P[1] end) or "C" (P[0], P[1] controls, P[2] end).
type Segment struct {
	Cmd byte
	P   []Point
}

// Path is a parsed stroke.
type Path struct {
	Start    Point
	Segments []Segment
}

var ErrEmpty = errors.New("empty path")

type token struct {
	cmd byte
	num float64
}

func tokenize(s string) ([]token, error) {
	var out []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == 'M' || c == 'L' || c == 'Q' || c == 'C':
			out = append(out, token{cmd: c})
			i++
		case c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9'):
			j := i
			if s[j] == '-' || s[j] == '+' {
				j++
			}
			digits := 0
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
				digits++
			}
			if j < len(s) && s[j] == '.' {
				j++
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
					digits++
				}
			}
			if digits == 0 {
				return nil, fmt.Errorf("bad number at %d", i)
			}
			v, err := strconv.ParseFloat(s[i:j], 64)
			if err != nil {
				return nil, fmt.Errorf("bad number at %d", i)
			}
			out = append(out, token{num: v})
			i = j
		default:
			return nil, fmt.Errorf("unsupported character %q at %d", c, i)
		}
	}
	return out, nil
}

var arity = map[byte]int{'M': 1, 'L': 1, 'Q': 2, 'C': 3}

// Parse parses and validates a stroke path.
func Parse(s string) (*Path, error) {
	toks, err := tokenize(s)
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 {
		return nil, ErrEmpty
	}
	if toks[0].cmd != 'M' {
		return nil, errors.New("path must start with M")
	}
	var p Path
	var cmd byte
	first := true
	i := 0
	for i < len(toks) {
		if toks[i].cmd != 0 {
			cmd = toks[i].cmd
			if cmd == 'M' && !first {
				return nil, errors.New("only one subpath is allowed")
			}
			i++
		} else if cmd == 'M' {
			// Extra coordinate pairs after M are implicit L commands (SVG).
			cmd = 'L'
		}
		n := arity[cmd]
		if i+2*n > len(toks) {
			return nil, fmt.Errorf("%c needs %d coordinates", cmd, 2*n)
		}
		pts := make([]Point, n)
		for k := 0; k < n; k++ {
			x, y := toks[i+2*k], toks[i+2*k+1]
			if x.cmd != 0 || y.cmd != 0 {
				return nil, fmt.Errorf("%c needs %d coordinates", cmd, 2*n)
			}
			if x.num < 0 || x.num > Grid || y.num < 0 || y.num > Grid {
				return nil, fmt.Errorf("coordinate (%g, %g) is outside the %d grid", x.num, y.num, Grid)
			}
			pts[k] = Point{x.num, y.num}
		}
		i += 2 * n
		if cmd == 'M' {
			p.Start = pts[0]
			first = false
			continue
		}
		p.Segments = append(p.Segments, Segment{Cmd: cmd, P: pts})
		// More numbers repeat the same command (SVG implicit repetition).
	}
	if len(p.Segments) == 0 {
		return nil, errors.New("path needs at least one segment after M")
	}
	return &p, nil
}
