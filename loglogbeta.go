package loglogbeta

import (
	"bytes"
	"encoding/gob"
	"math"

	bits "github.com/dgryski/go-bits"
	metro "github.com/dgryski/go-metro"
)

const (
	precision = 14
	m         = uint32(1 << precision)
	max       = 64 - precision
	maxX      = math.MaxUint64 >> max
	version   = 1
)

func beta(ez float64) float64 {
	zl := math.Log(ez + 1)
	return -0.370393911*ez +
		0.070471823*zl +
		0.17393686*math.Pow(zl, 2) +
		0.16339839*math.Pow(zl, 3) +
		-0.09237745*math.Pow(zl, 4) +
		0.03738027*math.Pow(zl, 5) +
		-0.005384159*math.Pow(zl, 6) +
		0.00042419*math.Pow(zl, 7)
}

func alpha(m float64) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	}
	return 0.7213 / (1 + 1.079/m)
}

func regSumAndZeros(registers [m]uint8) (float64, float64) {
	sum, ez := 0.0, 0.0
	for _, val := range registers {
		if val == 0 {
			ez++
		}
		sum += 1.0 / math.Pow(2.0, float64(val))
	}
	return sum, ez
}

func getPosVal(x uint64) (uint64, uint8) {
	val := uint8(bits.Clz((x<<precision)^maxX)) + 1
	k := x >> uint(max)
	return k, val
}

// LogLogBeta is a sketch for cardinality estimation based on LogLog counting
type LogLogBeta struct {
	registers [m]uint8
	alpha     float64
}

type savedLLB struct {
	Registers [m]uint8
	Alpha     float64
	Version   int
}

// New returns a LogLogBeta
func New() *LogLogBeta {
	return &LogLogBeta{
		registers: [m]uint8{},
		alpha:     alpha(float64(m)),
	}
}

// AddHash ...
func (llb *LogLogBeta) AddHash(x uint64) {
	k, val := getPosVal(x)
	if llb.registers[k] < val {
		llb.registers[k] = val
	}
}

// Add inserts a value into the sketch
func (llb *LogLogBeta) Add(value []byte) {
	x := metro.Hash64(value, 1337)
	llb.AddHash(x)
}

// Cardinality returns the number of unique elements added to the sketch
func (llb *LogLogBeta) Cardinality() uint64 {
	sum, ez := regSumAndZeros(llb.registers)
	m := float64(m)
	return uint64(llb.alpha * m * (m - ez) / (beta(ez) + sum))
}

// Merge takes another LogLogBeta and combines it with llb one, making llb the union of both.
func (llb *LogLogBeta) Merge(other *LogLogBeta) {
	for i, v := range llb.registers {
		if v < other.registers[i] {
			llb.registers[i] = other.registers[i]
		}
	}
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (llb *LogLogBeta) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(savedLLB{
		Version:   version,
		Alpha:     llb.alpha,
		Registers: llb.registers})

	return buf.Bytes(), err
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (llb *LogLogBeta) UnmarshalBinary(data []byte) error {
	// Unmarshal version. We may need this in the future if we make
	// non-compatible changes.

	var sllb savedLLB
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&sllb)

	if err != nil {
		return err
	}

	llb.registers = sllb.Registers
	llb.alpha = sllb.Alpha

	return nil

}
