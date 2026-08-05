package convert

import (
	"testing"
)

type data struct {
	TTLMillis int `json:"ttl"`
}

func TestJSON(t *testing.T) {
	d := &data{
		TTLMillis: 57600000,
	}

	m, err := EncodeToMap(d)
	if err != nil {
		t.Fatal(err)
	}

	i, _ := ToNumber(m["ttl"])
	if i != 57600000 {
		t.Fatal("not", 57600000, "got", m["ttl"])
	}

}
