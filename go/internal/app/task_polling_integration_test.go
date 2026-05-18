package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLongRunningTaskFlowOverMCPRoute(t *testing.T) {
	server := newTask3TestServer()
	stateDir := t.TempDir()
	server.cfg.StateDir = stateDir

	command := integrationSlowCommand()
	startReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_command_stream","command":"` + escapeJSON(command) + `","timeout":60}}`)
	startRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("submit status = %d, want %d", startRec.Code, http.StatusOK)
	}

	var startPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				TaskID string `json:"task_id"`
				Status string `json:"status"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("json.Unmarshal() start error = %v", err)
	}
	if startPayload.Result.IsError {
		t.Fatal("run_command_stream must succeed")
	}
	taskID := startPayload.Result.StructuredContent.TaskID
	if taskID == "" {
		t.Fatal("task_id must not be empty")
	}

	lastEventSeq := int64(0)
	deadline := time.Now().Add(10 * time.Second)
	var latestStatus string
	for time.Now().Before(deadline) {
		waitReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait_task","task_id":"` + taskID + `","timeout_seconds":1,"last_event_seq":` + int64ToString(lastEventSeq) + `}}`)
		waitRec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(waitRec, waitReq)
		if waitRec.Code != http.StatusOK {
			t.Fatalf("wait_task status = %d, want %d", waitRec.Code, http.StatusOK)
		}

		var waitPayload struct {
			Result struct {
				IsError           bool `json:"isError"`
				StructuredContent struct {
					TaskID                  string `json:"task_id"`
					Status                  string `json:"status"`
					EventSeq                int64  `json:"event_seq"`
					RecommendedPollStrategy string `json:"recommended_poll_strategy"`
				} `json:"structuredContent"`
			} `json:"result"`
		}
		if err := json.Unmarshal(waitRec.Body.Bytes(), &waitPayload); err != nil {
			t.Fatalf("json.Unmarshal() wait_task error = %v", err)
		}
		if waitPayload.Result.IsError {
			t.Fatal("wait_task must succeed")
		}
		if waitPayload.Result.StructuredContent.TaskID != taskID {
			t.Fatalf("wait_task task_id = %q, want %q", waitPayload.Result.StructuredContent.TaskID, taskID)
		}

		latestStatus = waitPayload.Result.StructuredContent.Status
		lastEventSeq = waitPayload.Result.StructuredContent.EventSeq
		if latestStatus == "succeeded" {
			if waitPayload.Result.StructuredContent.RecommendedPollStrategy != "stop" {
				t.Fatalf("recommended_poll_strategy = %q, want stop for terminal task", waitPayload.Result.StructuredContent.RecommendedPollStrategy)
			}
			break
		}
	}
	if latestStatus != "succeeded" {
		t.Fatalf("latest status = %q, want succeeded before deadline", latestStatus)
	}

	getReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_task","task_id":"` + taskID + `"}}`)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get_task status = %d, want %d", getRec.Code, http.StatusOK)
	}

	var getPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				TaskID                  string `json:"task_id"`
				Status                  string `json:"status"`
				RecommendedPollStrategy string `json:"recommended_poll_strategy"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("json.Unmarshal() get_task error = %v", err)
	}
	if getPayload.Result.IsError {
		t.Fatal("get_task must succeed")
	}
	if getPayload.Result.StructuredContent.Status != "succeeded" {
		t.Fatalf("get_task status = %q, want succeeded", getPayload.Result.StructuredContent.Status)
	}
	if getPayload.Result.StructuredContent.RecommendedPollStrategy != "stop" {
		t.Fatalf("get_task recommended_poll_strategy = %q, want stop", getPayload.Result.StructuredContent.RecommendedPollStrategy)
	}

	logsReq := newAuthorizedMCPRequest(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_task_logs","task_id":"` + taskID + `","stream":"stdout","offset":0,"limit":4096}}`)
	logsRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(logsRec, logsReq)
	if logsRec.Code != http.StatusOK {
		t.Fatalf("get_task_logs status = %d, want %d", logsRec.Code, http.StatusOK)
	}

	var logsPayload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Success bool   `json:"success"`
				Content string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(logsRec.Body.Bytes(), &logsPayload); err != nil {
		t.Fatalf("json.Unmarshal() get_task_logs error = %v", err)
	}
	if logsPayload.Result.IsError {
		t.Fatal("get_task_logs must succeed")
	}
	if !logsPayload.Result.StructuredContent.Success {
		t.Fatal("get_task_logs success = false, want true")
	}
	if !strings.Contains(logsPayload.Result.StructuredContent.Content, "start") || !strings.Contains(logsPayload.Result.StructuredContent.Content, "done") {
		t.Fatalf("stdout logs = %q, want both start and done markers", logsPayload.Result.StructuredContent.Content)
	}
}

func integrationSlowCommand() string {
	if runtime.GOOS == "windows" {
		return `powershell -NoProfile -Command "Write-Output start; Start-Sleep -Milliseconds 300; Write-Output done"`
	}
	return `printf 'start\n'; sleep 0.3; printf 'done\n'`
}

func escapeJSON(input string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(input)
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
