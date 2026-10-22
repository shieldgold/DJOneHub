package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSIMRecoveryReadyAndOtherLocksNeverSubmitPIN(t *testing.T) {
	for _, state := range []string{"READY", "SIM PUK", "SIM PIN2", "NOT INSERTED"} {
		t.Run(state, func(t *testing.T) {
			loaded := false
			r := &simRecovery{marker: filepath.Join(t.TempDir(), "pending"), load: func() (pinCredential, error) { loaded = true; return pinCredential{}, nil }}
			err := r.ensure(func(cmd string, _ time.Duration) (string, error) {
				if cmd != "AT+CPIN?" {
					t.Fatalf("unexpected command: %s", cmd)
				}
				return "+CPIN: " + state + "\r\nOK", nil
			})
			if loaded {
				t.Fatal("read credential for non-PIN state")
			}
			if state == "READY" && err != nil {
				t.Fatal(err)
			}
			if state != "READY" && err == nil {
				t.Fatal("unsafe SIM state reported ready")
			}
		})
	}
}

func TestSIMRecoveryUnlockOnceAndPersistFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		dir := t.TempDir()
		marker := filepath.Join(dir, "pending")
		attempts := 0
		ready := false
		load := func() (pinCredential, error) { return pinCredential{PIN: "1234", ICCID: "8900000000000000001"}, nil }
		command := func(cmd string, _ time.Duration) (string, error) {
			switch cmd {
			case "AT+CPIN?":
				if ready {
					return "+CPIN: READY\r\nOK", nil
				}
				return "+CPIN: SIM PIN\r\nOK", nil
			case "AT+QCCID":
				return "+QCCID: 8900000000000000001\r\nOK", nil
			default:
				attempts++
				if _, e := os.Stat(marker); e != nil {
					t.Fatal("PIN submitted before persistent guard")
				}
				if fail {
					return "", errors.New("transport failed 1234")
				}
				ready = true
				return "AT+CPIN=\"1234\"\r\nOK", nil
			}
		}
		r := &simRecovery{marker: marker, load: load}
		err := r.ensure(command)
		if fail && (err == nil || strings.Contains(err.Error(), "1234")) {
			t.Fatal("missing or unsafe failure")
		}
		if !fail && err != nil {
			t.Fatal(err)
		}
		// Simulate service restart: a failed/unknown submission must remain blocked.
		r = &simRecovery{marker: marker, load: load}
		_ = r.ensure(command)
		if attempts != 1 {
			t.Fatalf("PIN attempts = %d", attempts)
		}
		if !fail {
			if _, e := os.Stat(marker); !os.IsNotExist(e) {
				t.Fatal("success did not clear guard")
			}
		}
	}
}

func TestSIMRecoveryWrongSIMAndMissingCredential(t *testing.T) {
	for _, scenario := range []string{"mismatch", "no identity", "no keychain"} {
		r := &simRecovery{marker: filepath.Join(t.TempDir(), "pending"), load: func() (pinCredential, error) {
			if scenario == "no keychain" {
				return pinCredential{}, errors.New("locked")
			}
			return pinCredential{PIN: "1234", ICCID: "8900000000000000001"}, nil
		}}
		err := r.ensure(func(cmd string, _ time.Duration) (string, error) {
			switch cmd {
			case "AT+CPIN?":
				return "+CPIN: SIM PIN\r\nOK", nil
			case "AT+QCCID":
				if scenario == "no identity" {
					return "ERROR", nil
				}
				return "+QCCID: 8900000000000000002\r\nOK", nil
			default:
				t.Fatal("submitted PIN for unverified SIM")
				return "", nil
			}
		})
		if err == nil {
			t.Fatal("unsafe state accepted")
		}
	}
}

func TestSIMRecoveryConcurrentRequests(t *testing.T) {
	r := &simRecovery{marker: filepath.Join(t.TempDir(), "pending"), load: func() (pinCredential, error) { return pinCredential{PIN: "1234", ICCID: "8900000000000000001"}, nil }}
	ready := false
	attempts := 0
	command := func(cmd string, _ time.Duration) (string, error) {
		switch cmd {
		case "AT+CPIN?":
			if ready {
				return "+CPIN: READY\r\nOK", nil
			}
			return "+CPIN: SIM PIN\r\nOK", nil
		case "AT+QCCID":
			return "+QCCID: 8900000000000000001\r\nOK", nil
		default:
			attempts++
			ready = true
			return "OK", nil
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.ensure(command); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if attempts != 1 {
		t.Fatalf("duplicate unlocks: %d", attempts)
	}
}
