package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSMSResponseRequiresAcknowledgement(t *testing.T) {
	for _, tt := range []struct {
		response  string
		transport error
		want      string
	}{
		{"+CMGS: 17\r\nOK", nil, "submitted"},
		{"+CMS ERROR: 500", nil, "failed"},
		{"ERROR", nil, "failed"},
		{"+CMGS: 17", nil, "unknown"},
		{"private PDU payload", errors.New("private payload"), "unknown"},
	} {
		err := checkSMSSubmission(tt.response, tt.transport)
		state := "submitted"
		if err != nil {
			var se *smsSubmissionError
			if !errors.As(err, &se) {
				t.Fatal(err)
			}
			state = se.State
			if strings.Contains(err.Error(), "private") {
				t.Fatal("payload leaked")
			}
		}
		if state != tt.want {
			t.Fatalf("state %s want %s", state, tt.want)
		}
	}
}

func TestSMSSendStateRejectsDuplicateAndRetainsResult(t *testing.T) {
	a := &app{}
	if !a.beginSMSSend() {
		t.Fatal("initial send rejected")
	}
	if a.beginSMSSend() {
		t.Fatal("concurrent send allowed")
	}
	a.finishSMSSend(0, &smsSubmissionError{State: "unknown", Message: "结果不确定"})
	s := a.smsSendSnapshot()
	if s.State != "unknown" || s.UpdatedAt.IsZero() {
		t.Fatalf("missing receipt: %+v", s)
	}
	if !a.beginSMSSend() {
		t.Fatal("explicit later send rejected")
	}
	a.finishSMSSend(1, nil)
	if a.smsSendSnapshot().State != "submitted" {
		t.Fatal("missing success receipt")
	}
}
