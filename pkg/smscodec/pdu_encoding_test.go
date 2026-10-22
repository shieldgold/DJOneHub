package smscodec

import (
	"strings"
	"testing"

	"github.com/warthog618/sms/encoding/tpdu"
	"github.com/warthog618/sms/encoding/ucs2"
)

func TestBuildSubmitTPDUsWithOptionsForcesUCS2(t *testing.T) {
	tpdus, _, err := BuildSubmitTPDUsWithOptions("10086", "hello", SubmitOptions{Encoding: SMSEncodingUCS2})
	if err != nil {
		t.Fatalf("BuildSubmitTPDUsWithOptions() error = %v", err)
	}
	if len(tpdus) != 1 {
		t.Fatalf("parts=%d want 1", len(tpdus))
	}

	pdu := &tpdu.TPDU{Direction: tpdu.MO}
	if err := pdu.UnmarshalBinary(tpdus[0]); err != nil {
		t.Fatalf("UnmarshalBinary() error = %v", err)
	}
	if pdu.DCS != tpdu.DcsUCS2Data {
		t.Fatalf("DCS=0x%02x want 0x%02x", byte(pdu.DCS), byte(tpdu.DcsUCS2Data))
	}
	if got, want := []byte(pdu.UD), ucs2.Encode([]rune("hello")); string(got) != string(want) {
		t.Fatalf("UD=%x want UCS2 %x", got, want)
	}
}

func TestBuildSubmitTPDUsKeepsAutoEncodingByDefault(t *testing.T) {
	tpdus, _, err := BuildSubmitTPDUsWithOptions("10086", "hello", SubmitOptions{})
	if err != nil {
		t.Fatalf("BuildSubmitTPDUs() error = %v", err)
	}
	if len(tpdus) != 1 {
		t.Fatalf("parts=%d want 1", len(tpdus))
	}

	pdu := &tpdu.TPDU{Direction: tpdu.MO}
	if err := pdu.UnmarshalBinary(tpdus[0]); err != nil {
		t.Fatalf("UnmarshalBinary() error = %v", err)
	}
	if pdu.DCS != 0x00 {
		t.Fatalf("DCS=0x%02x want auto GSM7 0x00", byte(pdu.DCS))
	}
}

func TestSubmitDestinationAddressFormat(t *testing.T) {
	for _, tc := range []struct {
		name, number string
		toa          byte
	}{
		{"short service", "10086", 0x81},
		{"long service", "10691859839103", 0x81},
		{"domestic mobile", "13800138000", 0x81},
		{"international mobile", "+8613800138000", 0x91},
		{"trimmed domestic", " 13800138000 ", 0x81},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, message := range []string{"YZ", strings.Repeat("验证", 100)} {
				parts, lengths, err := BuildSubmitTPDUsWithOptions(tc.number, message, SubmitOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for i, part := range parts {
					p := &tpdu.TPDU{Direction: tpdu.MO}
					if err := p.UnmarshalBinary(part); err != nil {
						t.Fatal(err)
					}
					if p.DA.TOA != tc.toa {
						t.Errorf("part %d TOA=%02x want %02x", i, p.DA.TOA, tc.toa)
					}
					if p.DA.Number() != strings.TrimSpace(tc.number) {
						t.Errorf("destination=%q want %q", p.DA.Number(), strings.TrimSpace(tc.number))
					}
					if lengths[i] != len(part) {
						t.Errorf("invalid TPDU length")
					}
				}
			}
		})
	}
}
