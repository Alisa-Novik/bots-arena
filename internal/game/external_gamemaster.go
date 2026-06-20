package game

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const externalGameMasterName = "coolio-external"

type ExternalGameMaster struct {
	Name     string
	Command  []string
	Timeout  time.Duration
	Fallback GameMasterAdvisor
}

func NewExternalGameMaster(command []string, timeout time.Duration, fallback GameMasterAdvisor) *ExternalGameMaster {
	if timeout <= 0 {
		timeout = time.Second
	}
	return &ExternalGameMaster{
		Name:     externalGameMasterName,
		Command:  append([]string(nil), command...),
		Timeout:  timeout,
		Fallback: fallback,
	}
}

func SplitGameMasterCommand(command string) []string {
	fields := strings.Fields(strings.TrimSpace(command))
	return append([]string(nil), fields...)
}

func (m *ExternalGameMaster) AdvisorName() string {
	if m == nil || m.Name == "" {
		return externalGameMasterName
	}
	return m.Name
}

func (m *ExternalGameMaster) Decide(obs MasterObservation) (MasterEvent, bool) {
	if m == nil || len(m.Command) == 0 {
		return m.fallback(obs, "external game master command is empty")
	}

	input, err := json.Marshal(obs)
	if err != nil {
		return m.fallback(obs, fmt.Sprintf("external observation encode failed: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.Command[0], m.Command[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return m.fallback(obs, "external game master timed out")
		}
		return m.fallback(obs, fmt.Sprintf("external game master failed: %v %s", err, strings.TrimSpace(stderr.String())))
	}

	var event MasterEvent
	if err := json.Unmarshal(stdout.Bytes(), &event); err != nil {
		return m.fallback(obs, fmt.Sprintf("external game master returned invalid JSON: %v", err))
	}
	if event.Kind == "" || event.Kind == "none" {
		return MasterEvent{}, false
	}
	if event.Tick == 0 {
		event.Tick = obs.Tick
	}
	if event.Thought == "" {
		event.Thought = "Coolio made a quiet adjustment."
	}
	if event.Reason == "" {
		event.Reason = "external Coolio decision"
	}
	return event, true
}

func (m *ExternalGameMaster) fallback(obs MasterObservation, reason string) (MasterEvent, bool) {
	if m == nil || m.Fallback == nil {
		return MasterEvent{}, false
	}
	event, ok := m.Fallback.Decide(obs)
	if !ok {
		return MasterEvent{}, false
	}
	if event.Reason == "" {
		event.Reason = reason
	} else {
		event.Reason = reason + "; fallback: " + event.Reason
	}
	if event.Thought == "" {
		event.Thought = "Coolio wire failed, mock fins handled it."
	}
	return event, true
}
