// Package guid is used to handle the non-standard UUID from the Microsoft Active Directory.
// The objectGUID is following the DSP0134 specification, described in the
// DMTF System Management BIOS (SMBIOS) Reference Specification document:
//
//   - https://www.dmtf.org/sites/default/files/standards/documents/DSP0134_3.4.0.pdf
//
// According to this spec the bytes for the time_low, time_mid and time_hi_and_version
// values follow the little endian format.
//
// The standard RFC4122 encoding for the UUID "00112233-4455-6677-8899-AABBCCDDEEFF" is:
//
//	00 11 22 33 44 55 66 77 88 99 AA BB CC DD EE FF
//
// The encoding specified in DSP0134 is:
//
//	33 22 11 00 55 44 77 66 88 99 AA BB CC DD EE FF
package guid

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var uuidRegex = regexp.MustCompile("(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

// GUID represent the UUID in the DSP0134 spec
type GUID []byte

// Bytes returns the underlying bytes value
func (g GUID) Bytes() []byte {
	return g
}

// String returns UUID string representation
func (g GUID) String() string {
	return g.UUID()
}

// UUID returns the UUID string representation: "00112233-4455-6677-8899-AABBCCDDEEFF"
func (g GUID) UUID() string {
	if len(g) != 16 {
		return ""
	}

	u := swap(g)

	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		u[:4], u[4:6], u[6:8], u[8:10], u[10:],
	)
}

// Hex returns the Hex string representation: "33 22 11 00 55 44 77 66 88 99 AA BB CC DD EE FF"
func (g GUID) Hex() string {
	hexesArr := hexes(g.Bytes())

	for i := range hexesArr {
		hexesArr[i] = strings.ToUpper(hexesArr[i])
	}

	return strings.Join(hexesArr, " ")
}

// New returns a GUID object
func New(encoded []byte) (GUID, error) {
	if len(encoded) != 16 {
		return nil, errors.New("cannot create GUID from encoded bytes: invalid length")
	}

	return GUID(encoded), nil
}

// Parse returns a GUID object from a RFC4122 UUID string
func Parse(uuid string) (GUID, error) {
	if !uuidRegex.MatchString(uuid) {
		return nil, errors.New("cannot parse UUID to objectGUID: invalid format")
	}

	uuid = strings.ReplaceAll(uuid, "-", "")
	uuidBytes, err := hex.DecodeString(uuid)
	if err != nil {
		return nil, fmt.Errorf("cannot decode uuid string '%s' to hex: %w", uuid, err)
	}

	return GUID(swap(uuidBytes)), nil
}

// Escape returns an escaped string format of the objectGUID that can be safely used
// through the LDAP search. Every byte has to be encoded in an hex string,
// and prefixed with the '\' character. If a byte has a hex encoded string of
// length 1 then it will be prefixed with a '0'.
func Escape(guid GUID) string {
	builder := strings.Builder{}

	hexArray := hexes(guid.Bytes())
	for _, hex := range hexArray {
		builder.WriteString(`\`)
		builder.WriteString(hex)
	}

	return builder.String()
}

// swap will return a new array with the first three "bytes blocks" reversed
func swap(u []byte) []byte {
	if len(u) != 16 {
		return u
	}

	return []byte{
		u[3], u[2], u[1], u[0], // reverse 0-4
		u[5], u[4], // reverse 4-5
		u[7], u[6], // reverse 6-7
		u[8], u[9], u[10], u[11], u[12], u[13], u[14], u[15], // keep 8-15
	}
}

// hexes returns a string array of the hex decoded values
func hexes(bytes []byte) []string {
	var hexes []string

	for _, b := range bytes {
		hexes = append(hexes, fmt.Sprintf("%02x", b))
	}

	return hexes
}
