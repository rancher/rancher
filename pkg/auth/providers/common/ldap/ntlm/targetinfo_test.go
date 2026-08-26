package ntlm

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeAVPairs(pairs ...avPair) []byte {
	out := []byte{}
	for _, p := range pairs {
		out = binary.LittleEndian.AppendUint16(out, p.ID)
		out = binary.LittleEndian.AppendUint16(out, uint16(len(p.Value)))
		out = append(out, p.Value...)
	}
	return out
}

func eolTerminated(pairs ...avPair) []byte {
	return append(encodeAVPairs(pairs...), 0x00, 0x00, 0x00, 0x00)
}

// mustParseTargetInfo is the bridge for tests that build a wire buffer but call
// a function taking parsed pairs.
func mustParseTargetInfo(t *testing.T, buf []byte) []avPair {
	t.Helper()

	pairs, err := parseTargetInfo(buf)
	require.NoError(t, err)
	return pairs
}

func TestParseTargetInfo(t *testing.T) {
	t.Parallel()

	buf := eolTerminated(
		avPair{ID: avNbDomainName, Value: utf16LE("FOO")},
		avPair{ID: avTimestamp, Value: make([]byte, 8)},
	)

	pairs, err := parseTargetInfo(buf)
	require.NoError(t, err)
	require.Len(t, pairs, 2, "the terminating EOL is not returned as a pair")
	assert.Equal(t, avNbDomainName, pairs[0].ID)
	assert.Equal(t, utf16LE("FOO"), pairs[0].Value)
	assert.Equal(t, avTimestamp, pairs[1].ID)
}

func TestParseTargetInfoRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		buf  []byte
	}{
		{name: "empty", buf: nil},
		{name: "missing eol", buf: encodeAVPairs(avPair{ID: avNbDomainName, Value: utf16LE("FOO")})},
		{name: "truncated pair header", buf: []byte{0x02, 0x00, 0x06}},
		{
			name: "length past end",
			buf:  []byte{0x02, 0x00, 0xFF, 0xFF, 0x41, 0x42},
		},
		{
			name: "trailing bytes after eol",
			buf:  append(eolTerminated(), 0x41),
		},
		{
			name: "duplicate channel bindings",
			buf: eolTerminated(
				avPair{ID: avChannelBindings, Value: make([]byte, 16)},
				avPair{ID: avChannelBindings, Value: make([]byte, 16)},
			),
		},
		{
			name: "duplicate flags",
			buf: eolTerminated(
				avPair{ID: avFlags, Value: []byte{0, 0, 0, 0}},
				avPair{ID: avFlags, Value: []byte{2, 0, 0, 0}},
			),
		},
		{
			name: "duplicate target name",
			buf: eolTerminated(
				avPair{ID: avTargetName, Value: utf16LE("a")},
				avPair{ID: avTargetName, Value: utf16LE("b")},
			),
		},
		{
			name: "duplicate timestamp",
			buf: eolTerminated(
				avPair{ID: avTimestamp, Value: make([]byte, 8)},
				avPair{ID: avTimestamp, Value: make([]byte, 8)},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTargetInfo(test.buf)
			require.Error(t, err)
		})
	}
}

func TestParseTargetInfoAllowsDuplicateUnknownPairs(t *testing.T) {
	t.Parallel()

	buf := eolTerminated(
		avPair{ID: avDNSComputerName, Value: utf16LE("dc1")},
		avPair{ID: avDNSComputerName, Value: utf16LE("dc2")},
	)

	pairs, err := parseTargetInfo(buf)
	require.NoError(t, err)
	assert.Len(t, pairs, 2)
}

func TestTransformTargetInfoUpserts(t *testing.T) {
	t.Parallel()

	cbt := [16]byte{0xAA, 0xBB}
	server := mustParseTargetInfo(t, eolTerminated(
		avPair{ID: avNbDomainName, Value: utf16LE("FOO")},
		avPair{ID: avTimestamp, Value: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
	))

	got, err := transformTargetInfo(server, cbt, true)
	require.NoError(t, err)

	pairs, err := parseTargetInfo(got)
	require.NoError(t, err)

	counts := map[uint16]int{}
	for _, p := range pairs {
		counts[p.ID]++
	}
	assert.Equal(t, 1, counts[avChannelBindings])
	assert.Equal(t, 1, counts[avTargetName])
	assert.Equal(t, 1, counts[avFlags])
	assert.Equal(t, 1, counts[avNbDomainName], "unmodified pairs are preserved")
	assert.Equal(t, 1, counts[avTimestamp], "the server timestamp is preserved")

	// The server's pairs keep their original order at the front.
	assert.Equal(t, avNbDomainName, pairs[0].ID)
	assert.Equal(t, utf16LE("FOO"), pairs[0].Value)
	assert.Equal(t, avTimestamp, pairs[1].ID)

	value, ok := findAVPair(pairs, avChannelBindings)
	require.True(t, ok)
	assert.Equal(t, cbt[:], value)

	value, ok = findAVPair(pairs, avTargetName)
	require.True(t, ok)
	assert.Empty(t, value, "no SPN is supplied, so the target name is present with a zero length value")

	value, ok = findAVPair(pairs, avFlags)
	require.True(t, ok)
	require.Len(t, value, 4)
	flags := binary.LittleEndian.Uint32(value)
	assert.Equal(t, avFlagMICPresent, flags&avFlagMICPresent, "the MIC present bit is set")
	assert.Zero(t, flags&0x00000004, "the unverified target name bit is never set")

	// Exactly one EOL, and it terminates the buffer.
	assert.Equal(t, []byte{0, 0, 0, 0}, got[len(got)-4:])
}

func TestTransformTargetInfoWithoutMIC(t *testing.T) {
	t.Parallel()

	got, err := transformTargetInfo(nil, [16]byte{}, false)
	require.NoError(t, err)

	pairs, err := parseTargetInfo(got)
	require.NoError(t, err)

	value, ok := findAVPair(pairs, avFlags)
	require.True(t, ok)
	assert.Zero(t, binary.LittleEndian.Uint32(value)&avFlagMICPresent)
}

func TestTransformTargetInfoReplacesExistingPairs(t *testing.T) {
	t.Parallel()

	cbt := [16]byte{0x11}
	server := mustParseTargetInfo(t, eolTerminated(
		avPair{ID: avTargetName, Value: utf16LE("ldap/dc1.example.com")},
		avPair{ID: avFlags, Value: []byte{0x01, 0x00, 0x00, 0x00}},
		avPair{ID: avChannelBindings, Value: make([]byte, 16)},
	))

	got, err := transformTargetInfo(server, cbt, true)
	require.NoError(t, err)

	pairs, err := parseTargetInfo(got)
	require.NoError(t, err)
	require.Len(t, pairs, 3, "each pair is replaced in place, not duplicated")

	value, _ := findAVPair(pairs, avTargetName)
	assert.Empty(t, value)
	value, _ = findAVPair(pairs, avChannelBindings)
	assert.Equal(t, cbt[:], value)
	value, _ = findAVPair(pairs, avFlags)
	assert.Equal(t, uint32(0x01)|avFlagMICPresent, binary.LittleEndian.Uint32(value),
		"existing flag bits are preserved and the MIC bit is added")
}

func TestSerializeTargetInfoRoundTrip(t *testing.T) {
	t.Parallel()

	pairs := []avPair{
		{ID: avNbDomainName, Value: utf16LE("FOO")},
		{ID: avTargetName, Value: []byte{}},
	}

	encoded, err := serializeTargetInfo(pairs)
	require.NoError(t, err)

	got, err := parseTargetInfo(encoded)
	require.NoError(t, err)
	assert.Equal(t, pairs, got)
}

func TestSerializeTargetInfoRejectsAnOversizedValue(t *testing.T) {
	t.Parallel()

	_, err := serializeTargetInfo([]avPair{
		{ID: avNbDomainName, Value: make([]byte, maxVarFieldLen+1)},
	})
	require.Error(t, err, "a 16-bit length cannot describe the value, so it must not be truncated")
}

func FuzzParseTargetInfo(f *testing.F) {
	f.Add(eolTerminated(avPair{ID: avNbDomainName, Value: utf16LE("FOO")}))
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, buf []byte) {
		pairs, err := parseTargetInfo(buf)
		if err != nil {
			return
		}
		// A buffer that parses must re-serialize to the same bytes.
		got, err := serializeTargetInfo(pairs)
		if err != nil {
			t.Fatalf("a buffer that parsed failed to re-serialize: %v", err)
		}
		if string(got) != string(buf) {
			t.Fatalf("round trip changed the buffer: %x -> %x", buf, got)
		}
	})
}
