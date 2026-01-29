package shell

import "strings"

type outputBuffer struct {
	lines []string
	start int
	size  int
	max   int
}

func newOutputBuffer(max int) *outputBuffer {
	if max < 0 {
		max = 0
	}
	return &outputBuffer{lines: make([]string, max), max: max}
}

func (b *outputBuffer) Append(lines ...string) {
	for _, line := range lines {
		if b.max == 0 {
			continue
		}
		if b.size < b.max {
			idx := (b.start + b.size) % b.max
			b.lines[idx] = line
			b.size++
			continue
		}
		b.lines[b.start] = line
		b.start = (b.start + 1) % b.max
	}
}

func (b *outputBuffer) Lines() []string {
	if b.size == 0 {
		return nil
	}
	out := make([]string, 0, b.size)
	for i := 0; i < b.size; i++ {
		idx := (b.start + i) % b.max
		out = append(out, b.lines[idx])
	}
	return out
}

func (b *outputBuffer) String() string {
	if b.size == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(b.size * 80)
	for i := 0; i < b.size; i++ {
		idx := (b.start + i) % b.max
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(b.lines[idx])
	}
	return sb.String()
}
