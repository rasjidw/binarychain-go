package binarychain

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	ZERO_SOP = 0x80
	EOC_BYTE = 0xFF
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

type BinaryChain interface {
	Serialise() []byte
}

type binaryChain struct {
	Prefix *string
	Parts  *[][]byte
}

func (bc *binaryChain) Serialise() *[]byte {
	result := []byte(*bc.Prefix)

	for i := 0; i < len(*bc.Parts); i++ {
		part := &(*bc.Parts)[i]
		result = slices.Concat(result, createPartLength(part), *part)
	}
	result = append(result, EOC_BYTE)
	return &result
}

func NewBinaryChain(prefix *string, parts *[][]byte) (*binaryChain, error) {
	if !isASCII(prefix) {
		return nil, errors.New("Prefix must be an ASCII string")
	} else {
		return &binaryChain{prefix, parts}, nil
	}
}

func (bc *binaryChain) String() string {
	var body string
	if len(*bc.Prefix) > 100 {
		// the prefix is ascii, so just taking a slice is fine.
		body = fmt.Sprintf("prefix=\"%v...\"", (*bc.Prefix)[:100])
	} else {
		body = fmt.Sprintf("prefix=\"%v\"", *bc.Prefix)
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

// structs for streaming reader
type BinaryChainItem interface {
	isBinaryChainItem()
}

// end of chain maker
type EndOfChainMarker struct{}

func (eoc EndOfChainMarker) isBinaryChainItem() {}

// bc prefix
type BinaryChainPrefix struct {
	Prefix string
}

func (prefix BinaryChainPrefix) isBinaryChainItem() {}

// bc part
type BinaryChainPart struct {
	Part []byte
}

func (part BinaryChainPart) isBinaryChainItem() {}
