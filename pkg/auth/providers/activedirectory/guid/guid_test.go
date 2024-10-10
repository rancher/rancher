package guid_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory/guid"
)

func TestDecodings(t *testing.T) {
	tt := []struct {
		name         string
		encoded      []byte
		expectedUUID string
		expectedHex  string
		expectedErr  string
	}{
		{
			name:         "valid objectGUID 1",
			encoded:      []byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb"),
			expectedUUID: "3d0ef6af-965b-44e3-8fea-b23a7d3aa6cb",
			expectedHex:  "AF F6 0E 3D 5B 96 E3 44 8F EA B2 3A 7D 3A A6 CB",
		},
		{
			name:         "valid objectGUID 2",
			encoded:      []byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15"),
			expectedUUID: "75593fbf-57d1-4c55-872d-9372ef0fdd15",
			expectedHex:  "BF 3F 59 75 D1 57 55 4C 87 2D 93 72 EF 0F DD 15",
		},
		{
			name:         "valid objectGUID 3",
			encoded:      []byte("\x36\xf4\x21\x9b\xf9\x8a\x54\x48\x95\x3d\xc6\x36\x99\x90\xe0\xa0"),
			expectedUUID: "9b21f436-8af9-4854-953d-c6369990e0a0",
			expectedHex:  "36 F4 21 9B F9 8A 54 48 95 3D C6 36 99 90 E0 A0",
		},
		{
			name:         "valid objectGUID with N char",
			encoded:      []byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e"),
			expectedUUID: "4e4e4e4e-4e4e-4e4e-4e4e-4e4e4e4e4e4e",
			expectedHex:  "4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E 4E",
		},
		{
			name:        "objectGUID with invalid length",
			encoded:     []byte("\xaf\xf6\x0e\x96\xe3"),
			expectedErr: "invalid length",
		},
		{
			// This test data was taken from the following MS example:
			// https://learn.microsoft.com/en-us/dotnet/api/system.guid.tobytearray?view=net-8.0
			name:         "Microsoft GUID",
			encoded:      []byte("\xC9\x8B\x91\x35\x6D\x19\xEA\x40\x97\x79\x88\x9D\x79\xB7\x53\xF0"),
			expectedUUID: "35918bc9-196d-40ea-9779-889d79b753f0",
			expectedHex:  "C9 8B 91 35 6D 19 EA 40 97 79 88 9D 79 B7 53 F0",
		},
		{
			name:         "nil GUID",
			encoded:      nil,
			expectedUUID: "",
			expectedHex:  "",
			expectedErr:  "invalid length",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			parsedGUID, err := guid.New(tc.encoded)

			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedUUID, parsedGUID.UUID())
				assert.Equal(t, tc.expectedHex, parsedGUID.Hex())
			}
		})
	}
}

func TestParse(t *testing.T) {
	tt := []struct {
		name         string
		uuid         string
		expectedGUID []byte
		expectedErr  string
	}{
		{
			name:         "valid uuid 1",
			uuid:         "3d0ef6af-965b-44e3-8fea-b23a7d3aa6cb",
			expectedGUID: []byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb"),
		},
		{
			name:         "valid uuid 2",
			uuid:         "75593fbf-57d1-4c55-872d-9372ef0fdd15",
			expectedGUID: []byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15"),
		},
		{
			name:         "valid uuid 3",
			uuid:         "9b21f436-8af9-4854-953d-c6369990e0a0",
			expectedGUID: []byte("\x36\xf4\x21\x9b\xf9\x8a\x54\x48\x95\x3d\xc6\x36\x99\x90\xe0\xa0"),
		},
		{
			name:         "valid uuid with N char",
			uuid:         "4e4e4e4e-4e4e-4e4e-4e4e-4e4e4e4e4e4e",
			expectedGUID: []byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e"),
		},
		{
			name:        "invalid uuid",
			uuid:        "75593fbf",
			expectedErr: "invalid format",
		},
		{
			name:        "empty uuid",
			uuid:        "",
			expectedErr: "invalid format",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			objectGUID, err := guid.Parse(tc.uuid)

			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedGUID, objectGUID.Bytes())
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tt := []struct {
		name        string
		objectGUID  guid.GUID
		escapedGUID string
	}{
		{
			name:        "valid objectGUID 1",
			objectGUID:  guid.GUID([]byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb")),
			escapedGUID: "\\af\\f6\\0e\\3d\\5b\\96\\e3\\44\\8f\\ea\\b2\\3a\\7d\\3a\\a6\\cb",
		},
		{
			name:        "valid objectGUID 2",
			objectGUID:  guid.GUID([]byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15")),
			escapedGUID: "\\bf\\3f\\59\\75\\d1\\57\\55\\4c\\87\\2d\\93\\72\\ef\\0f\\dd\\15",
		},
		{
			name:        "valid objectGUID with N char",
			objectGUID:  guid.GUID([]byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e")),
			escapedGUID: "\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e",
		},
		{
			name:        "short objectGUID",
			objectGUID:  guid.GUID([]byte("a")),
			escapedGUID: "\\61",
		},
		{
			name:        "empty objectGUID",
			objectGUID:  guid.GUID([]byte("")),
			escapedGUID: "",
		},
		{
			name:        "nil objectGUID",
			escapedGUID: "",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			escaped := guid.Escape(tc.objectGUID)
			assert.Equal(t, tc.escapedGUID, escaped)
		})
	}
}
