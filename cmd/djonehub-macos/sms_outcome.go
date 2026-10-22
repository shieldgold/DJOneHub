package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type smsSubmissionError struct {
	State   string
	Message string
}

func (e *smsSubmissionError) Error() string { return e.Message }

type smsSendOutcome struct {
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
	Segments  int       `json:"segments"`
	Error     string    `json:"error,omitempty"`
}

func checkSMSSubmission(response string, transport error) error {
	if transport != nil {
		return &smsSubmissionError{State: "unknown", Message: "短信提交结果不确定（通信中断或超时），请核对接收端，不要直接重发"}
	}
	if atResponseIsError(response) || strings.TrimSpace(response) == "ERROR" {
		code := regexp.MustCompile(`\+(?:CMS|CME) ERROR:\s*\d+`).FindString(response)
		if code == "" {
			code = "ERROR"
		}
		return &smsSubmissionError{State: "failed", Message: "模块拒绝发送：" + code}
	}
	if !strings.Contains(response, "+CMGS:") || !atProbeSucceeded(response) {
		return &smsSubmissionError{State: "unknown", Message: "未收到完整发送确认，结果不确定，请核对接收端"}
	}
	return nil
}

func (a *app) beginSMSSend() bool {
	a.smsOutcomeMu.Lock()
	defer a.smsOutcomeMu.Unlock()
	if a.smsOutcome.State == "sending" {
		return false
	}
	a.smsOutcome = smsSendOutcome{State: "sending", UpdatedAt: time.Now()}
	return true
}
func (a *app) finishSMSSend(segments int, err error) smsSendOutcome {
	a.smsOutcomeMu.Lock()
	defer a.smsOutcomeMu.Unlock()
	s := smsSendOutcome{State: "submitted", Segments: segments, UpdatedAt: time.Now()}
	if err != nil {
		s.State = "failed"
		s.Error = err.Error()
		var se *smsSubmissionError
		if errors.As(err, &se) {
			s.State = se.State
		}
		if segments > 0 {
			s.Error = fmt.Sprintf("已有 %d 个分片提交成功；%s。不要整条重发", segments, s.Error)
		}
	}
	a.smsOutcome = s
	return s
}
func (a *app) smsSendSnapshot() smsSendOutcome {
	a.smsOutcomeMu.Lock()
	defer a.smsOutcomeMu.Unlock()
	return a.smsOutcome
}
