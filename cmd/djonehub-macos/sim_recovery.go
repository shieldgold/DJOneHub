package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type pinCredential struct {
	PIN   string `json:"pin"`
	ICCID string `json:"iccid"`
}
type simRecovery struct {
	mu     sync.Mutex
	marker string
	load   func() (pinCredential, error)
	status string
}

// A guard is persisted before submitting a PIN. A crash, wrong PIN or unknown
// outcome therefore cannot consume more retries after a service restart.
func (r *simRecovery) ensure(command func(string, time.Duration) (string, error)) (result error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer func() {
		if result != nil {
			r.status = result.Error()
		} else {
			r.status = "SIM 已就绪"
		}
	}()
	response, err := command("AT+CPIN?", 3*time.Second)
	if err != nil {
		return errors.New("无法读取 SIM 状态，等待模块重新连接")
	}
	state := parseUSBATPrefixed(response, "+CPIN:")
	if state == "READY" {
		return nil
	}
	if state != "SIM PIN" {
		return errors.New("SIM 未就绪或需要人工处理（未提交 PIN）")
	}
	if _, err = os.Stat(r.marker); !os.IsNotExist(err) {
		return errors.New("自动解锁已暂停：上次 PIN 提交失败或结果不确定，请人工核对后解除保护")
	}
	credential, err := r.load()
	if err != nil {
		return errors.New("SIM 需要 PIN，但无法读取钥匙串中的自动解锁凭据")
	}
	if !regexp.MustCompile(`^[0-9]{4,8}$`).MatchString(credential.PIN) || !regexp.MustCompile(`^[0-9]{18,22}$`).MatchString(credential.ICCID) {
		return errors.New("钥匙串中的 SIM 凭据格式无效")
	}
	identity, err := command("AT+QCCID", 3*time.Second)
	if err != nil || parseUSBATPrefixed(identity, "+QCCID:") != credential.ICCID {
		return errors.New("SIM 与已保存的卡片不匹配或暂时无法识别，未提交 PIN")
	}
	if r.marker == "" {
		return errors.New("无法定位用户目录，未提交 PIN")
	}
	if err = os.MkdirAll(filepath.Dir(r.marker), 0700); err != nil {
		return errors.New("无法创建 PIN 重试保护目录，未提交 PIN")
	}
	file, err := os.OpenFile(r.marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("无法创建 PIN 重试保护文件，未提交 PIN")
	}
	_, writeErr := file.WriteString("PIN submission pending; inspect SIM before removing this guard.\n")
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("PIN 重试保护保存失败，未提交 PIN")
	}
	// Never wrap the transport error or response: they can contain an echoed PIN.
	response, err = command(`AT+CPIN="`+credential.PIN+`"`, 10*time.Second)
	if err != nil || !atProbeSucceeded(response) {
		return errors.New("PIN 解锁失败或结果不确定；自动重试已暂停，不会再次消耗次数")
	}
	for i := 0; i < 5; i++ {
		response, err = command("AT+CPIN?", 3*time.Second)
		if err == nil && parseUSBATPrefixed(response, "+CPIN:") == "READY" {
			if err = os.Remove(r.marker); err != nil {
				return errors.New("SIM 已解锁，但重试保护文件无法清除")
			}
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return errors.New("PIN 已提交但 SIM 尚未确认就绪，自动重试已暂停")
}

func loadPINFromKeychain() (pinCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-a", "sim-auto-unlock", "-s", "local.djonehub.macos", "-w").Output()
	if err != nil {
		return pinCredential{}, errors.New("keychain unavailable")
	}
	var credential pinCredential
	if json.Unmarshal(data, &credential) != nil {
		return pinCredential{}, errors.New("invalid credential")
	}
	return credential, nil
}

func (a *app) initSIMRecovery() {
	a.simRecoveryOnce.Do(func() {
		home, err := os.UserHomeDir()
		marker := ""
		if err == nil {
			marker = filepath.Join(home, "Library", "Application Support", "DJOneHub", "pin-unlock.pending")
		}
		a.simRecovery = &simRecovery{marker: marker, load: loadPINFromKeychain}
	})
}
func (a *app) ensureSIMReady() error {
	a.initSIMRecovery()
	return a.simRecovery.ensure(a.runATCommand)
}
func (a *app) simRecoveryStatus() string {
	a.initSIMRecovery()
	a.simRecovery.mu.Lock()
	defer a.simRecovery.mu.Unlock()
	return strings.TrimSpace(a.simRecovery.status)
}
