// Package uu - Simple uuencode/uudecode
//
// This package provides simple uuencoding and uudecoding.  It is compatible
// with Perl's pack/unpack "u" template. See
// https://perldoc.perl.org/functions/pack for more details.
package uu

/*
 * uu.go
 * Simple uuencode/uudecode
 * By J. Stuart McMurray
 * Created 20240928
 * Last Modified 20240928
 */

import (
	"bytes"
	"slices"
)

const (
	// lineLen is the number of unencoded bytes to put in an encoded line.
	lineLen = 45
	// uuOffset is the offset from a number to a character.
	uuOffset = 32
	// encChunkLen is the number of encoded bytes in a chunk.
	encChunkLen = 4
	// decChunkLen is the number of decoded bytes in a chunk.
	decChunkLen = 3
)

// AppendDecode decodes src, appends it to dst, and returns the extended
// buffer.
func AppendDecode(dst, src []byte) ([]byte, error) {
	/* Decode each line. */
	for lineN, line := range bytes.Split(src, []byte{'\n'}) {
		/* Ignore blank lines. */
		if 0 == len(line) {
			continue
		}
		/* Ignore \r's. */
		if '\r' == line[len(line)-1] {
			line = line[:len(line)-1]
		}
		/* Make sure the encode part is a multiple of four bytes. */
		if 0b00 != (len(line)-1)&0b11 {
			return nil, DecodeError{
				Line: lineN,
				Err:  ErrInvalidDataLen,
			}
		}
		/* Work out how many bytes to grab. */
		var nDec int
		if '`' != line[0] { /* ` is a zero-byte line. */
			if line[0] < uuOffset {
				return nil, DecodeError{
					Line: lineN,
					Err: InvalidLengthCharacterError(
						line[0],
					),
				}
			}
			nDec = int(line[0] - uuOffset)
		}

		/* Make sure we have as many as we expect. */
		encLen := nDec / decChunkLen
		if 0 != nDec%decChunkLen {
			encLen++
		}
		encLen *= encChunkLen
		if len(line)-1 != encLen {
			return nil, DecodeError{
				Line: lineN,
				Err: IncorrectDataLenError{
					Expected: encLen,
					Actual:   len(line) - 1,
				},
			}
		}

		/* Decode the line. */
		var offset = 1 /* After length byte. */
		nDecRem := nDec
		for chunk := range slices.Chunk(line[1:], encChunkLen) {
			/* Sanitize '`s. */
			if slices.Contains(chunk, '`') {
				chunk = bytes.Clone(chunk)
				for i, v := range chunk {
					if '`' == v {
						chunk[i] = uuOffset
					}
				}
			}
			/* Make sure all of the characters are usable. */
			for i, v := range chunk {
				if v < uuOffset || (0b111111+uuOffset) < v {
					err := InvalidEncodedCharacterError(v)
					return nil, DecodeError{
						Line:   lineN,
						Offset: offset + i,
						Err:    err,
					}
				}
			}
			dec := []byte{
				(chunk[0]-uuOffset)<<2 +
					(chunk[1]-uuOffset)>>4,
				(chunk[1]-uuOffset)<<4 +
					(chunk[2]-uuOffset)>>2,
				(chunk[2]-uuOffset)<<6 +
					(chunk[3] - uuOffset),
			}
			/* Save the decoded bits, less padding. */
			if len(dec) > nDecRem {
				dec = dec[:nDecRem]
			}
			dst = append(dst, dec...)
			/* Bookkeeping. */
			offset += encChunkLen
			nDecRem -= len(dec)
		}
	}

	return dst, nil
}

// AppendEncode encodes src, appends it to dst, and returns the extended
// buffer.
func AppendEncode(dst, src []byte) []byte {
	/* Encode in 45-byte chunks. */
	for line := range slices.Chunk(src, lineLen) {
		/* Work out the size. */
		dst = append(dst, byte(uuOffset+len(line)))
		/* Encode the data itself. */
		for chunk := range slices.Chunk(line, decChunkLen) {
			/* Make sure we have enough chunk. */
			switch len(chunk) {
			case decChunkLen: /* Good. */
			case 2: /* Need to add a byte. */
				chunk = append(chunk, 0)
			case 1: /* Need to add a couple of bytes. */
				chunk = append(chunk, 0, 0)
			}
			/* Turn into four bytes. */
			enc := [encChunkLen]byte{
				chunk[0] >> 2,
				chunk[0]<<4 | chunk[1]>>4,
				chunk[1]<<2 | chunk[2]>>6,
				chunk[2] & 0b00111111,
			}
			for i, v := range enc {
				enc[i] = 0b00111111 & v
			}
			/* Encode to UU standards. */
			for i := range enc {
				if 0 == enc[i] {
					enc[i] = '`'
				} else {
					enc[i] += uuOffset
				}
			}
			/* Save this chunk. */
			dst = append(dst, enc[0], enc[1], enc[2], enc[3])
		}
		/* Finished the line. */
		dst = append(dst, '\n')
	}
	return dst
}

// MaxDecodedLen eeturns a maximum number of bytes of the decoded form of b as
// required by AppendDecode.
// Accuracy is traded for O(1) execution; in practice AppendDecode will usuallu
// require less space.
func MaxDecodedLen(b []byte) int {
	return 1 + (len(b) * 16 / decChunkLen)
}

// MaxEncodedLen eeturns a maximum number of bytes of the encoded form of b as
// required by AppendEncode.
// Accuracy is traded for O(1) execution; in practice AppendEncode will usuallu
// require less space.
func MaxEncodedLen(b []byte) int {
	return 63 * (1 + (len(b) / lineLen))
}
