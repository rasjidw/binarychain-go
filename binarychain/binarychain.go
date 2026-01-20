package binarychain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"math"
	"slices"
	"unicode"
)

// from https://stackoverflow.com/questions/53069040/checking-a-string-contains-only-ascii-characters
// there is a faster version (but longer / more complicated version) on that answer
func isASCII(s *string) bool {
	for i := 0; i < len(*s); i++ {
		if (*s)[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

const (
	ZERO_SOP        = 0x80
	EOC_BYTE        = 0xFF
	MAX_LENGTH_SIZE = 8
)

func createPartLength(part *[]byte) []byte {
	l := uint64(len(*part))

	if l == 0 {
		return []byte{ZERO_SOP}
	}

	intbuf := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(intbuf, l)

	fullbuf := make([]byte, binary.MaxVarintLen64+1)
	seen_sig := false
	j := 1
	for i := 0; i < 8; i++ {
		current := intbuf[i]
		if current > 0 && !seen_sig {
			seen_sig = true
		}
		if seen_sig {
			fullbuf[j] = current
			j++
		}
	}
	fullbuf[0] = byte(ZERO_SOP + j - 1)
	return fullbuf[:j]
}

type BinaryChain struct {
	prefix *string
	Parts  *[]([]byte)
}

func (bc *BinaryChain) SetPrefix(prefix *string) error {
	if isASCII(prefix) {
		bc.prefix = prefix
		return nil
	} else {
		return errors.New("Prefix must be an ASCII string")
	}
}

func (bc *BinaryChain) GetPrefix() *string {
	return bc.prefix
}

func (bc *BinaryChain) Serialise() *[]byte {
	result := []byte(*bc.prefix)

	for i := 0; i < len(*bc.Parts); i++ {
		part := &(*bc.Parts)[i]
		result = slices.Concat(result, createPartLength(part), *part)
	}
	result = append(result, EOC_BYTE)
	return &result
}

func NewBinaryChain(prefix *string, parts *[][]byte) (*BinaryChain, error) {
	if !isASCII(prefix) {
		return nil, errors.New("Prefix must be an ASCII string")
	} else {
		return &BinaryChain{prefix, parts}, nil
	}
}

func (bc *BinaryChain) String() string {
	var body string
	if len(*bc.prefix) > 100 {
		// the prefix is ascii, so just taking a slice is fine.
		body = fmt.Sprintf("prefix=\"%v...\"", (*bc.prefix)[:100])
	} else {
		body = fmt.Sprintf("prefix=\"%v\"", *bc.prefix)
	}

	body += ", parts=["
	if len(*bc.Parts) == 0 {
		body += "]"
	} else {
		var parts_to_show [][]byte
		var suffix string
		if len(*bc.Parts) > 10 {
			parts_to_show = (*bc.Parts)[:10]
			suffix = "...]"
		} else {
			parts_to_show = *bc.Parts
			suffix = "]"
		}
		for i := 0; i < len(parts_to_show); i++ {
			part := parts_to_show[i]
			if i != 0 {
				body += ", "
			}
			body += fmt.Sprintf("0x%x", part)
		}
		body += suffix
	}

	return fmt.Sprintf("BinaryChain<{%v}>", body)
}

// -----------------------------------

// structs for streaming reader
type BinaryChainItem interface {
	isBinaryChainItem()
}

// end of chain maker
type EndOfChainMarker struct{}

func (eoc *EndOfChainMarker) isBinaryChainItem() {}

func (eoc *EndOfChainMarker) String() string {
	return "<EndOfChainMarker>"
}

// bc prefix
type BinaryChainPrefix struct {
	Prefix string
}

func (prefix *BinaryChainPrefix) isBinaryChainItem() {}

func (prefix *BinaryChainPrefix) String() string {
	var txt string
	if len(prefix.Prefix) < 100 {
		txt = prefix.Prefix
	} else {
		txt := prefix.Prefix[:100]
		txt += "..."
	}
	return fmt.Sprintf("<BCPrefix: %v>", txt)
}

// bc part
type BinaryChainPart struct {
	Part []byte
}

func (part *BinaryChainPart) isBinaryChainItem() {}

func (part *BinaryChainPart) String() string {
	var txt string
	if len(part.Part) < 30 {
		txt = fmt.Sprintf("0x%x", part.Part)
	} else {
		txt = fmt.Sprintf("0x%x ...", part.Part[:30])
	}
	return fmt.Sprintf("<BCPart: %v>", txt)
}

// parse error
type ParseError struct {
	message string
}

func (pe *ParseError) Error() string {
	return fmt.Sprintf("Parse Error: %s", pe.message)
}

// func (pe *ParseError) isBinaryChainItem() {}

// ---- base struct for both streaming and non-streaming readers

type chainReaderConf struct {
	maxPrefixSize  int
	maxPartSize    int
	maxChainSize   int // total size of the chain (includig / excluding? length bytes?)
	maxChainLength int
}

func (crc *chainReaderConf) SetMaxPrefixSize(maxPrefixSize int) error {
	if maxPrefixSize < 0 {
		return errors.New("maxPrefixSize must be greater or equal to 0")
	}
	crc.maxPrefixSize = maxPrefixSize
	return nil
}

func (crc *chainReaderConf) MaxPrefixSize() int {
	return crc.maxPrefixSize
}

func (crc *chainReaderConf) SetMaxPartSize(maxPartSize int) error {
	if maxPartSize <= 0 {
		return errors.New("maxPartSize must be greater than 0")
	}
	crc.maxPartSize = maxPartSize
	return nil
}

func (crc *chainReaderConf) MaxPartSize() int {
	return crc.maxPartSize
}

func (crc *chainReaderConf) SetMaxChainSize(maxChainSize int) error {
	if maxChainSize <= 0 {
		return errors.New("maxChainSize must be greater than 0")
	}
	crc.maxChainSize = maxChainSize
	return nil
}

func (crc *chainReaderConf) MaxChainSize() int {
	return crc.maxChainSize
}

func (crc *chainReaderConf) SetMaxChainLength(maxChainLength int) error {
	if maxChainLength <= 0 {
		return errors.New("maxChainLength must be greater than 0")
	}
	crc.maxChainLength = maxChainLength
	return nil
}

func (crc *chainReaderConf) MaxChainLength() int {
	return crc.maxChainLength
}

// -----------------------------------------

const DEFAULT_MAX_PREFIX = 256

// ---- streaming reader

// parsing state for streaming reader
type ParsingState int

const (
	IN_PREFIX ParsingState = iota
	IN_PART_LENGTH
	IN_BINARY_PART
)

var stateName = map[ParsingState]string{
	IN_PREFIX:      "IN_PREFIX",
	IN_PART_LENGTH: "IN_PART_LENGTH",
	IN_BINARY_PART: "IN_BINARY_PART",
}

func (ps ParsingState) String() string {
	return stateName[ps]
}

type streamingChainReader struct {
	Conf             *chainReaderConf
	state            ParsingState
	buffer           []byte
	curPrefixOffset  int
	partLengthSize   int // -1 = unknown
	binaryPartLength int // -1 = unknown
	chainSize        int
	chainLength      int
	returnAtEnd      bool
}

func (scr *streamingChainReader) init() {
	scr.partLengthSize = -1
	scr.binaryPartLength = -1
	scr.chainLength = -1
}

func NewStreamingChainReader() *streamingChainReader {
	crc := chainReaderConf{maxPrefixSize: DEFAULT_MAX_PREFIX, maxPartSize: math.MaxInt, maxChainSize: math.MaxInt, maxChainLength: math.MaxInt}
	scr := streamingChainReader{Conf: &crc}
	scr.init()
	return &scr
}

func (scr *streamingChainReader) AddData(newData []byte) error {
	if len(newData) == 0 {
		return errors.New("Must add at least one byte")
	}
	if scr.chainSize+len(newData) > scr.Conf.maxChainSize {
		return errors.New("Chain too large")
	}
	// scr.buffer = append(scr.buffer, newData...)
	scr.buffer = slices.Concat(scr.buffer, newData)
	return nil
}

type itemResult struct {
	gotResult    bool
	Result       BinaryChainItem
	atEndOfChain bool
	ItemErr      error
}

func (scr *streamingChainReader) GetNextItem() itemResult {
	if scr.returnAtEnd {
		scr.returnAtEnd = false
		return itemResult{gotResult: true, Result: &EndOfChainMarker{}}
	}
	partResult := scr.getNextPart()
	// if not result or an error, just return without further processing
	if !partResult.gotResult || partResult.ItemErr != nil {
		return partResult
	}
	scr.chainLength += 1
	if scr.Conf.maxChainLength >= 0 && scr.chainLength >= scr.Conf.maxChainLength && !partResult.atEndOfChain {
		// already at the max length, so if not at end of the chain, raise an error
		return itemResult{ItemErr: &ParseError{"chain too long"}}
	}
	if partResult.atEndOfChain {
		scr.returnAtEnd = true // return EndOfChainMarker next call
		scr.chainSize = 0
		scr.chainLength = -1
	}
	return partResult
}

func (scr *streamingChainReader) getNextPart() itemResult {
	switch scr.state {
	case IN_PREFIX:
		return scr.getPrefix()
	case IN_PART_LENGTH:
		err := scr.readPartLength() // may change the state
		if err != nil {
			return itemResult{ItemErr: err}
		}
		if scr.state == IN_BINARY_PART {
			return scr.getBinaryPart()
		} else {
			return itemResult{}
		}
	case IN_BINARY_PART:
		return scr.getBinaryPart()
	default:
		return itemResult{ItemErr: errors.New("Invalid state!")}
	}
}

func (scr *streamingChainReader) getPrefix() itemResult {
	for i := scr.curPrefixOffset; i < len(scr.buffer); i++ {
		b := scr.buffer[i]
		if b >= ZERO_SOP {
			prefix := BinaryChainPrefix{string(scr.buffer[:i])}
			atEnd, err := scr.setStateAndPartLengthSize(b)
			if err != nil {
				return itemResult{ItemErr: err}
			}
			scr.buffer = scr.buffer[i+1:]
			return itemResult{gotResult: true, Result: &prefix, atEndOfChain: atEnd}
		}
	}
	return itemResult{}
}

func (scr *streamingChainReader) setStateAndPartLengthSize(sop_byte byte) (bool, error) {
	// return if at end, sets the state
	if sop_byte == EOC_BYTE {
		scr.partLengthSize = -1
		scr.state = IN_PREFIX
		return true, nil
	} else if ZERO_SOP <= sop_byte && sop_byte <= ZERO_SOP+MAX_LENGTH_SIZE {
		scr.partLengthSize = int(sop_byte) - ZERO_SOP
		if scr.partLengthSize == 0 {
			scr.binaryPartLength = 0
			scr.state = IN_BINARY_PART
		} else {
			scr.state = IN_PART_LENGTH
		}
		return false, nil
	} else {
		return false, &ParseError{"Invalid start of part byte"}
	}
}

func (scr *streamingChainReader) readPartLength() error {
	if scr.partLengthSize == -1 {
		return errors.New("Invalid call - partLengthSize is unknown (-1)")
	}
	if scr.partLengthSize <= len(scr.buffer) {
		encodedPartLength := scr.buffer[:scr.partLengthSize]
		paddedPartLength := []byte{0, 0, 0, 0, 0, 0, 0, 0}
		paddedPartLength = slices.Concat(paddedPartLength[:8-len(encodedPartLength)], encodedPartLength)
		scr.buffer = scr.buffer[scr.partLengthSize:]
		scr.state = IN_BINARY_PART
		scr.binaryPartLength = int(binary.BigEndian.Uint64(paddedPartLength))
		if scr.binaryPartLength > scr.Conf.maxPartSize {
			return &ParseError{"Part length too long"}
		}
	}
	return nil
}

func (scr *streamingChainReader) getBinaryPart() itemResult {
	if scr.binaryPartLength == -1 {
		return itemResult{ItemErr: errors.New("Invalid call: binaryPartLength is unknown (-1)")}
	}
	// include the SOP / EOC byte in the length requried
	data_length := scr.binaryPartLength + 1
	if data_length <= len(scr.buffer) {
		// got all the data needed in the buffer
		part_plus_end := scr.buffer[:data_length]
		binary_part := part_plus_end[:data_length-1]
		sop_byte := part_plus_end[data_length-1]
		atEnd, err := scr.setStateAndPartLengthSize(sop_byte)
		if err != nil {
			return itemResult{ItemErr: err}
		}
		scr.buffer = scr.buffer[data_length:]
		return itemResult{gotResult: true, Result: &BinaryChainPart{binary_part}, atEndOfChain: atEnd}
	}
	return itemResult{}
}

func (scr *streamingChainReader) GetChainParts(newData []byte) iter.Seq[itemResult] {
	return func(yield func(itemResult) bool) {
		err := scr.AddData(newData)
		if err != nil {
			itemResult := itemResult{ItemErr: err}
			if !yield(itemResult) {
				return
			}
		} else {
			for {
				itemResult := scr.GetNextItem()
				if itemResult.Result == nil && itemResult.ItemErr == nil {
					return
				}
				if !yield(itemResult) {
					return
				}
			}
		}
	}
}

// --- non-streaming chain reader

type readerResult struct {
	BinChain  *BinaryChain
	ErrResult *error
}

type chainReader struct {
	Conf *chainReaderConf
	scr  *streamingChainReader
	bc   *BinaryChain
}

func NewChainReader() *chainReader {
	crc := chainReaderConf{maxPrefixSize: DEFAULT_MAX_PREFIX, maxPartSize: math.MaxInt, maxChainSize: math.MaxInt,
		maxChainLength: math.MaxInt}
	scr := streamingChainReader{Conf: &crc}
	scr.init()
	prefix := ""
	empty_parts := []([]byte){}
	bc, err := NewBinaryChain(&prefix, &empty_parts)
	if err != nil {
		panic("This should not happen")
	}
	cr := chainReader{Conf: &crc, scr: &scr, bc: bc}
	return &cr
}

func (cr *chainReader) GetBinaryChains(newData []byte) iter.Seq[readerResult] {
	return func(yield func(readerResult) bool) {
		for item := range cr.scr.GetChainParts(newData) {
			if item.ItemErr != nil {
				errResult := readerResult{ErrResult: &item.ItemErr}
				yield(errResult)
				return
			}

			switch v := item.Result.(type) {
			case *BinaryChainPrefix:
				// fmt.Printf("Got Prefix: %v\n", v)
				cr.bc.prefix = &v.Prefix
			case *BinaryChainPart:
				// fmt.Printf("Got Binary Part: %v\n", v)
				newParts := *cr.bc.Parts
				newParts = append(newParts, v.Part)
				cr.bc.Parts = &newParts
			case *EndOfChainMarker:
				// fmt.Printf("Got EOC Marker: %v\n", v)
				bc_result := readerResult{BinChain: cr.bc}
				if !yield(bc_result) {
					return
				}
				prefix := ""
				empty_parts := []([]byte){}
				new_bc, err := NewBinaryChain(&prefix, &empty_parts)
				if err != nil {
					panic("This should not happen")
				}
				cr.bc = new_bc
			default:
				// should never get here
				fmt.Printf("Default case: Got %v\n", v)
			}
		}
	}
}
