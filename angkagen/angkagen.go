// Package angkagen generates Belajar Angka "Hitung Benda" questions.
//
// Questions are never stored: a session keeps only a seed, and this
// deterministic generator turns (level settings, object list, seed) into the
// same questions on the server, in the app (lib/core/angka, Dart) and in the
// backoffice preview. It uses only 32-bit integer arithmetic, so the Go and
// Dart versions agree bit for bit; testdata/generator_golden.json is the
// parity fixture both test suites replay.
package angkagen

// Area is the logical box pictures are placed in; clients scale it to fit.
const (
	AreaWidth  = 320
	AreaHeight = 180
	// Gap is the minimum free space between two pictures.
	Gap = 12
	// ScatterTries is how many candidates a picture gets before the
	// question falls back to rows.
	ScatterTries = 30
	// RowMax is the most pictures in one row.
	RowMax = 5
)

const (
	LayoutScatter = "scatter"
	LayoutRows    = "rows"
)

// Rand is mulberry32.
type Rand struct{ state uint32 }

func NewRand(seed uint32) *Rand { return &Rand{state: seed} }

// Next returns the next 32-bit value.
func (r *Rand) Next() uint32 {
	r.state += 0x6D2B79F5
	t := r.state
	t = (t ^ (t >> 15)) * (t | 1)
	t = (t + (t^(t>>7))*(t|61)) ^ t
	return t ^ (t >> 14)
}

// Intn returns a value in [0, n).
func (r *Rand) Intn(n int) int { return int(r.Next() % uint32(n)) }

func (r *Rand) shuffle(s []int) {
	for i := len(s) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

// Params are a level's generation settings plus a session's seed and object
// snapshot.
type Params struct {
	Min       int      `json:"min"`
	Max       int      `json:"max"`
	Count     int      `json:"count"`
	ObjectIDs []string `json:"object_ids"`
	Layout    string   `json:"layout"`
	Seed      uint32   `json:"seed"`
}

// Point is a picture's centre in the AreaWidth × AreaHeight box.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Question is one generated question.
type Question struct {
	Count    int     `json:"count"`
	ObjectID string  `json:"object_id"`
	Size     int     `json:"size"`
	Layout   string  `json:"layout"`
	Points   []Point `json:"points"`
}

// PictureSize is the picture side for a question with n pictures: RowMax
// per row always fit, and scatter usually succeeds (≥ 85% of questions up to
// 10 pictures).
func PictureSize(n int) int {
	switch {
	case n <= 5:
		return 52
	case n <= 10:
		return 38
	case n <= 15:
		return 30
	default:
		return 26
	}
}

// Generate returns p.Count questions. The PRNG is used in a fixed order: the
// remainder shuffle, the bag shuffle, then per question its object and its
// positions.
func Generate(p Params) []Question {
	r := NewRand(p.Seed)
	counts := countBag(r, p.Min, p.Max, p.Count)
	out := make([]Question, len(counts))
	prev := -1
	for i, n := range counts {
		obj := ""
		if len(p.ObjectIDs) > 0 {
			idx := r.Intn(len(p.ObjectIDs))
			if idx == prev && len(p.ObjectIDs) > 1 {
				idx = (idx + 1) % len(p.ObjectIDs)
			}
			prev = idx
			obj = p.ObjectIDs[idx]
		}
		size := PictureSize(n)
		layout := LayoutRows
		var points []Point
		if p.Layout == LayoutScatter {
			if pts, ok := scatter(r, n, size); ok {
				points, layout = pts, LayoutScatter
			}
		}
		if points == nil {
			points = rows(n, size)
		}
		out[i] = Question{Count: n, ObjectID: obj, Size: size, Layout: layout, Points: points}
	}
	return out
}

// countBag holds n/R full copies of min..max plus the first n%R values of a
// shuffled copy, shuffled, then spread so equal counts aren't neighbours.
func countBag(r *Rand, min, max, n int) []int {
	if n <= 0 || max < min {
		return nil
	}
	span := max - min + 1
	full := make([]int, span)
	for i := range full {
		full[i] = min + i
	}
	bag := make([]int, 0, n)
	for c := 0; c < n/span; c++ {
		bag = append(bag, full...)
	}
	if rem := n % span; rem > 0 {
		extra := append([]int(nil), full...)
		r.shuffle(extra)
		bag = append(bag, extra[:rem]...)
	}
	r.shuffle(bag)
	return spreadRepeats(bag)
}

// spreadRepeats re-reads bag in order, each step taking the first remaining
// value that differs from the previous one — unless one value fills more
// than half of what remains, which must then go next. No two neighbours are
// equal whenever such an order exists (only R = 1 forces repeats).
func spreadRepeats(bag []int) []int {
	rest := append([]int(nil), bag...)
	out := make([]int, 0, len(bag))
	for len(rest) > 0 {
		pick := -1
		counts := map[int]int{}
		for _, v := range rest {
			counts[v]++
		}
		for i, v := range rest {
			if counts[v]*2 > len(rest) {
				pick = i
				break
			}
		}
		if pick < 0 {
			pick = 0
			for i, v := range rest {
				if len(out) == 0 || v != out[len(out)-1] {
					pick = i
					break
				}
			}
		}
		out = append(out, rest[pick])
		rest = append(rest[:pick], rest[pick+1:]...)
	}
	return out
}

// scatter places n pictures by dart throwing; ok is false when one picture
// found no free spot in ScatterTries candidates.
func scatter(r *Rand, n, size int) ([]Point, bool) {
	minDist := (size + Gap) * (size + Gap)
	half := size / 2
	pts := make([]Point, 0, n)
	for k := 0; k < n; k++ {
		placed := false
		for try := 0; try < ScatterTries; try++ {
			c := Point{
				X: half + r.Intn(AreaWidth-size+1),
				Y: half + r.Intn(AreaHeight-size+1),
			}
			free := true
			for _, q := range pts {
				dx, dy := c.X-q.X, c.Y-q.Y
				if dx*dx+dy*dy < minDist {
					free = false
					break
				}
			}
			if free {
				pts = append(pts, c)
				placed = true
				break
			}
		}
		if !placed {
			return nil, false
		}
	}
	return pts, true
}

// rows lays n pictures out in ceil(n/RowMax) centred rows, earlier rows
// taking the extra picture (7 → 4 + 3).
func rows(n, size int) []Point {
	if n <= 0 {
		return []Point{}
	}
	lines := (n + RowMax - 1) / RowMax
	base, extra := n/lines, n%lines
	height := lines*size + (lines-1)*Gap
	y := (AreaHeight-height)/2 + size/2
	pts := make([]Point, 0, n)
	for l := 0; l < lines; l++ {
		k := base
		if l < extra {
			k++
		}
		width := k*size + (k-1)*Gap
		x := (AreaWidth-width)/2 + size/2
		for i := 0; i < k; i++ {
			pts = append(pts, Point{X: x + i*(size+Gap), Y: y})
		}
		y += size + Gap
	}
	return pts
}
